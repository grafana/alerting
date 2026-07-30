package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	authnlib "github.com/grafana/authlib/authn"
	authtypes "github.com/grafana/authlib/types"
	"github.com/grafana/grafana-app-sdk/app"
	"github.com/grafana/grafana-app-sdk/logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/grafana/alerting/apps/historian/pkg/apis/alertinghistorian/v0alpha1"
	"github.com/grafana/alerting/apps/historian/pkg/app/config"
)

type Notification struct {
	loki   *LokiReader
	logger logging.Logger

	// rbacEnabled indicates results must be restricted to the accessible folders.
	rbacEnabled bool
	// folderAccess resolves the folder UIDs whose alert rules the requester can
	// read. When rbacEnabled is true this must be non-nil for queries to be served.
	folderAccess folderAccessReader
	// authenticator reconstructs the caller identity from the tokens forwarded on a
	// request (X-Access-Token / X-Grafana-Id) when the context does not already
	// carry it. It is only set when running standalone (the historian operator),
	// whose custom-route server does not authenticate requests. In-process (behind
	// the aggregated API server) the identity is already in the context and this is
	// nil.
	authenticator authnlib.Authenticator
	// folderAPIRemote indicates the folder list targets a remote folder API (a
	// dedicated FolderAPIConfig was supplied) rather than the app's loopback kube
	// config. A remote REST client does not carry the request-context identity over
	// the wire, so the caller's identity headers must be forwarded explicitly on the
	// folder request even when the identity is already in the context (authenticator
	// nil). See forwardedIdentityHeaders.
	folderAPIRemote bool
}

func New(cfg config.NotificationConfig, kubeConfig rest.Config, reg prometheus.Registerer, logger logging.Logger, tracer trace.Tracer) *Notification {
	if !cfg.Enabled {
		return &Notification{}
	}

	n := &Notification{
		loki:        NewLokiReader(cfg.Loki, reg, logger, tracer),
		logger:      logger,
		rbacEnabled: cfg.RBACEnabled,
	}

	if cfg.RBACEnabled {
		// Use the dedicated folder API config when supplied (split multi-apiserver
		// deployment, where the app's own API server does not serve
		// folder.grafana.app); otherwise fall back to the app's kube config, which
		// in-process is a loopback serving every group.
		folderConfig := kubeConfig
		if cfg.FolderAPIConfig != nil {
			folderConfig = *cfg.FolderAPIConfig
			n.folderAPIRemote = true
		}

		reader, err := newFolderAccessReader(folderConfig, cfg.AccessClient, logger)
		if err != nil {
			// Leave folderAccess nil; handlers fail closed when RBAC is enabled but
			// the reader could not be constructed.
			logger.Error("Failed to construct folder access reader; RBAC-protected notification queries will be rejected", "err", err)
		} else {
			n.folderAccess = reader
		}

		// When a signing keys URL is configured the app is running standalone and
		// must reconstruct the caller identity from the forwarded tokens, since the
		// operator's custom-route server does not authenticate requests. Mirrors
		// Grafana's extended-JWT client: the access token audience is validated, ID
		// tokens are not audience-scoped.
		if cfg.SigningKeysURL != "" {
			keys := authnlib.NewKeyRetriever(authnlib.KeyRetrieverConfig{SigningKeysURL: cfg.SigningKeysURL})
			n.authenticator = authnlib.NewDefaultAuthenticator(
				authnlib.NewAccessTokenVerifier(authnlib.VerifierConfig{AllowedAudiences: jwt.Audience(cfg.AllowedAudiences)}, keys),
				authnlib.NewIDTokenVerifier(authnlib.VerifierConfig{}, keys),
			)
		}
	}

	return n
}

// resolveRuleFilter returns the RBAC rule filter for the request. It returns a
// nil filter when RBAC is disabled (no filtering). When RBAC is enabled it
// resolves the folders whose alert rules the requester can read and returns a
// filter restricting results to those folders.
func (n *Notification) resolveRuleFilter(ctx context.Context, namespace string, headers http.Header) (*ruleFilter, error) {
	if !n.rbacEnabled {
		return nil, nil
	}
	if n.folderAccess == nil {
		return nil, errors.New("folder access reader is not configured")
	}
	ctx, err := n.contextWithAuthInfo(ctx, headers)
	if err != nil {
		return nil, err
	}
	if err := requireDelegatedUserIdentity(ctx); err != nil {
		return nil, err
	}
	folders, err := n.folderAccess.AccessibleFolders(ctx, namespace, n.forwardedIdentityHeaders(headers))
	if err != nil {
		return nil, err
	}
	return newRuleFilter(folders), nil
}

// errServiceIdentityForbidden marks a request authenticated as a bare service
// (access-policy) identity rather than an end user. Notification history is
// user-scoped: authorization must reflect the caller's own folder ACLs, not a
// service token's embedded scope. authlib returns the raw access-token
// permissions (authtypes.AuthInfo.GetTokenPermissions) precisely when the
// identity type is TypeAccessPolicy, so a service JWT carrying a broad
// alertrules:list scope would otherwise read history across every folder,
// bypassing user RBAC. Such a caller is authenticated but not permitted here, so
// it surfaces as 403 Forbidden.
var errServiceIdentityForbidden = errors.New("service identity is not permitted to query user-scoped notification history")

// requireDelegatedUserIdentity rejects callers whose authorization would be based
// on a service token's own scope instead of a delegated end-user identity. It
// applies in both deployment modes (in-process and standalone): the identity is
// read from the context after contextWithAuthInfo has populated it. A caller
// acting on behalf of a user (an ID token, or an access token embedding a user
// actor) resolves to a non-access-policy type and is allowed, because authz then
// evaluates the user's folder ACLs rather than the token scope. When no identity
// is present the check is a no-op and downstream RBAC fails closed.
func requireDelegatedUserIdentity(ctx context.Context) error {
	info, ok := authtypes.AuthInfoFrom(ctx)
	if !ok {
		return nil
	}
	if info.GetIdentityType() == authtypes.TypeAccessPolicy {
		return errServiceIdentityForbidden
	}
	return nil
}

// headerAccessToken and headerGrafanaID carry the caller's signed access and ID
// tokens. They identify the acting user to Grafana's aggregated API servers and
// are the same headers the authenticator reads to reconstruct the identity.
const (
	headerAccessToken = "X-Access-Token"
	headerGrafanaID   = "X-Grafana-Id"
)

// forwardedIdentityHeaders returns the caller identity headers to attach to
// outbound folder API requests. Injecting the reconstructed identity into the
// context is enough for the AccessClient (BatchCheck takes the identity
// explicitly), but the folder list goes through the kube REST client, which does
// not read that identity. Headers are forwarded in two cases:
//
//   - Standalone (the historian operator, authenticator non-nil): its kube client
//     authenticates as the operator's own service account, so without forwarding
//     these headers folder enumeration would run under the operator identity and
//     skip the caller's folders:read check.
//   - Remote folder API (folderAPIRemote, a dedicated FolderAPIConfig was set):
//     the REST client opens a fresh connection to another API server and does not
//     carry the request-context identity over the wire, so it must be forwarded
//     even though the identity is already in the context.
//
// In-process the app's loopback kube client preserves the request-context
// identity via its in-memory transport, so nil is returned and nothing is added.
func (n *Notification) forwardedIdentityHeaders(headers http.Header) http.Header {
	if n.authenticator == nil && !n.folderAPIRemote {
		return nil
	}
	out := http.Header{}
	if v := headers.Get(headerAccessToken); v != "" {
		out.Set(headerAccessToken, v)
	}
	if v := headers.Get(headerGrafanaID); v != "" {
		out.Set(headerGrafanaID, v)
	}
	return out
}

// errUnauthenticated marks a failure to reconstruct the caller identity from the
// tokens forwarded on a standalone request (missing, malformed, expired, or
// otherwise invalid X-Access-Token / X-Grafana-Id). It is a client error and
// must surface as 401 Unauthorized rather than being reported as a server fault,
// which would invite retries and mask the auth problem. Failures that are not the
// caller's fault (e.g. the JWKS endpoint being unreachable) are not wrapped and
// remain internal server errors.
var errUnauthenticated = errors.New("unauthenticated")

// contextWithAuthInfo ensures the context carries the caller identity used for
// RBAC. Behind the aggregated API server the identity is authenticated upstream
// and already present, so the context is returned unchanged. Running standalone
// (the historian operator), the custom-route server does not authenticate
// requests, so the identity is reconstructed from the forwarded access/ID tokens
// (X-Access-Token / X-Grafana-Id) and injected. When no authenticator is
// configured the context is returned unchanged and downstream RBAC rejects the
// request for lack of identity.
func (n *Notification) contextWithAuthInfo(ctx context.Context, headers http.Header) (context.Context, error) {
	if _, ok := authtypes.AuthInfoFrom(ctx); ok {
		return ctx, nil
	}
	if n.authenticator == nil {
		return ctx, nil
	}
	provider := authnlib.NewHTTPTokenProvider(&http.Request{Header: headers})
	info, err := n.authenticator.Authenticate(ctx, provider)
	if err != nil {
		// A bad or missing token is the caller's fault (401); mark it so the
		// handlers do not report it as an internal server error. Other failures
		// (e.g. the signing keys could not be fetched) stay internal.
		if authnlib.IsUnauthenticatedErr(err) {
			return nil, fmt.Errorf("%w: %w", errUnauthenticated, err)
		}
		return nil, fmt.Errorf("authenticate request: %w", err)
	}
	return authtypes.WithAuthInfo(ctx, info), nil
}

// ruleFilterStatusError maps a resolveRuleFilter failure to the appropriate API
// status. A failure to reconstruct the caller identity from the forwarded tokens
// is a client error (401 Unauthorized); a request authenticated as a bare service
// identity is 403 Forbidden; everything else is an internal server error. Keeping
// these client cases out of the 500 path stops callers and monitors from treating
// an auth problem as a server fault.
func (n *Notification) ruleFilterStatusError(msg string, err error, start time.Time) *apierrors.StatusError {
	if errors.Is(err, errUnauthenticated) {
		n.logger.Debug(msg, "err", err, "duration", time.Since(start))
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Reason:  metav1.StatusReasonUnauthorized,
				Code:    http.StatusUnauthorized,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}
	if errors.Is(err, errServiceIdentityForbidden) {
		n.logger.Debug(msg, "err", err, "duration", time.Since(start))
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Reason:  metav1.StatusReasonForbidden,
				Code:    http.StatusForbidden,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}
	n.logger.Error(msg, "err", err, "duration", time.Since(start))
	return &apierrors.StatusError{
		ErrStatus: metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("%s: %s", msg, err.Error()),
		}}
}

func (n *Notification) QueryAlertsHandler(ctx context.Context, writer app.CustomRouteResponseWriter, request *app.CustomRouteRequest) error {
	start := time.Now()

	if n.loki == nil {
		const msg = "Notification alerts query whilst disabled"
		n.logger.Debug(msg)
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusUnprocessableEntity,
				Message: msg,
			}}
	}

	var body v0alpha1.CreateNotificationsqueryalertsRequestBody
	err := json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		const msg = "Notification alerts query malformed"
		n.logger.Debug(msg, "err", err)
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}

	filter, err := n.resolveRuleFilter(ctx, request.ResourceIdentifier.Namespace, request.Headers)
	if err != nil {
		return n.ruleFilterStatusError("Notification alerts query authorization failed", err, start)
	}

	response, err := n.loki.QueryAlerts(ctx, body, filter)
	if err != nil {
		if errors.Is(err, ErrInvalidQuery) {
			const msg = "Notification alerts query invalid"
			n.logger.Debug(msg, "err", err)
			return &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusBadRequest,
					Message: fmt.Sprintf("%s: %s", msg, err.Error()),
				}}
		}
		const msg = "Notification alerts query failed"
		n.logger.Error(msg, "err", err, "duration", time.Since(start))
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}

	n.logger.Debug("Notification alerts query success",
		"alerts", len(response.Alerts),
		"duration", time.Since(start))

	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	return json.NewEncoder(writer).Encode(response)
}

func (n *Notification) QueryHandler(ctx context.Context, writer app.CustomRouteResponseWriter, request *app.CustomRouteRequest) error {
	start := time.Now()

	if n.loki == nil {
		const msg = "Notification history query whilst disabled"
		n.logger.Debug(msg)
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusUnprocessableEntity,
				Message: msg,
			}}
	}

	var body v0alpha1.CreateNotificationqueryRequestBody
	err := json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		const msg = "Notification history query malformed"
		n.logger.Debug(msg, "err", err)
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}

	filter, err := n.resolveRuleFilter(ctx, request.ResourceIdentifier.Namespace, request.Headers)
	if err != nil {
		return n.ruleFilterStatusError("Notification history query authorization failed", err, start)
	}

	response, err := n.loki.Query(ctx, body, filter)
	if err != nil {
		if errors.Is(err, ErrInvalidQuery) {
			const msg = "Notification history query invalid"
			n.logger.Debug(msg, "err", err)
			return &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusBadRequest,
					Message: fmt.Sprintf("%s: %s", msg, err.Error()),
				}}
		}
		const msg = "Notification history query failed"
		n.logger.Error(msg, "err", err, "duration", time.Since(start))
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}

	n.logger.Debug("Notification history query success",
		"entries", len(response.Entries),
		"counts", len(response.Counts),
		"duration", time.Since(start))

	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	return json.NewEncoder(writer).Encode(response)
}
