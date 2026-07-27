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
		reader, err := newFolderAccessReader(kubeConfig, cfg.AccessClient, logger)
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
	folders, err := n.folderAccess.AccessibleFolders(ctx, namespace, n.forwardedIdentityHeaders(headers))
	if err != nil {
		return nil, err
	}
	return newRuleFilter(folders), nil
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
// not read that identity. Running standalone (the historian operator) its kube
// client authenticates as the operator's own service account, so without
// forwarding these headers folder enumeration would run under the operator
// identity and skip the caller's folders:read check. Behind the aggregated API
// server (authenticator nil) the kube client already forwards the caller identity
// from the request context, so nil is returned and nothing extra is attached.
func (n *Notification) forwardedIdentityHeaders(headers http.Header) http.Header {
	if n.authenticator == nil {
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
		return nil, fmt.Errorf("authenticate request: %w", err)
	}
	return authtypes.WithAuthInfo(ctx, info), nil
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
		const msg = "Notification alerts query authorization failed"
		n.logger.Error(msg, "err", err, "duration", time.Since(start))
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
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
		const msg = "Notification history query authorization failed"
		n.logger.Error(msg, "err", err, "duration", time.Since(start))
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
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
