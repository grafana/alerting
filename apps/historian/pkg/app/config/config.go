package config

import (
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/pflag"

	"github.com/grafana/alerting/notify/historian/lokiclient"
	authtypes "github.com/grafana/authlib/types"
	"github.com/grafana/grafana-app-sdk/simple"
	"k8s.io/client-go/rest"
)

const (
	lokiDefaultMaxQueryLength = 721 * time.Hour // 30d1h, matches the default value in Loki
	lokiDefaultMaxQuerySize   = 65536           // 64kb
)

type LokiConfig struct {
	lokiclient.LokiConfig
	Transport http.RoundTripper
}

type NotificationConfig struct {
	Enabled bool
	Loki    LokiConfig
	// RBACEnabled restricts notification history results to the folders whose
	// alert rules the requesting user is allowed to read. When enabled, the app
	// lists the tenant's folders via the multi-tenant folder API and then confirms
	// alert.rules:read on each folder via AccessClient, keeping only the folders
	// the caller may see. Results are filtered to those folders.
	RBACEnabled bool
	// AccessClient authorizes alert.rules:read per folder for RBAC filtering. It is
	// set programmatically by the deployment wiring (not via a flag). When
	// RBACEnabled is true it must be non-nil, otherwise notification queries fail
	// closed.
	AccessClient authtypes.AccessClient
	// SigningKeysURL is the JWKS endpoint used to verify the access/ID tokens
	// forwarded on notification requests when the app runs standalone (as the
	// historian operator). Behind the aggregated API server the caller identity is
	// authenticated upstream and placed in the request context; the standalone
	// operator's custom-route server does not authenticate requests, so identity
	// must be reconstructed from the forwarded tokens. Leave empty when running
	// in-process (identity is already present in the context).
	SigningKeysURL string
	// AllowedAudiences restricts which audiences the forwarded access token may
	// carry (typically the historian's own API audience). Empty disables audience
	// validation. Only used when SigningKeysURL is set.
	AllowedAudiences []string
	// FolderAPIConfig, when set, is used to build the folder API client for RBAC
	// folder enumeration instead of the app's own kube config. It is set
	// programmatically by the deployment wiring (not via a flag).
	//
	// In-process (and behind the aggregated API server) the app's kube config is a
	// loopback that serves every group, so folder.grafana.app is reachable and the
	// caller identity is preserved by the in-memory transport; leave this nil. In a
	// split multi-apiserver deployment the historian's own API server does not
	// serve folder.grafana.app, so the folder list must target the remote folder
	// app. When set, the caller's identity headers are also forwarded on the folder
	// request (see forwardedIdentityHeaders) because a remote REST client does not
	// carry the request-context identity over the wire.
	FolderAPIConfig *rest.Config
}

type RuntimeConfig struct {
	GetAlertStateHistoryHandler simple.AppCustomRouteHandler
	Notification                NotificationConfig
}

func (n *NotificationConfig) AddFlagsWithPrefix(prefix string, flags *pflag.FlagSet) {
	flags.BoolVar(&n.Enabled, prefix+".enabled", false, "Enable notification query endpoints")
	flags.BoolVar(&n.RBACEnabled, prefix+".rbac-enabled", false, "Restrict notification history to the alert rules the requesting user can access")
	flags.StringVar(&n.SigningKeysURL, prefix+".rbac.signing-keys-url", "", "JWKS URL used to verify the access/ID tokens forwarded on notification requests when running standalone (the historian operator). Leave empty when running in-process, where the caller identity is already present in the request context")
	flags.StringSliceVar(&n.AllowedAudiences, prefix+".rbac.allowed-audiences", nil, "Comma-separated list of audiences the forwarded access token must carry (typically the historian API audience). Empty disables audience validation")
	addLokiFlags(&n.Loki.LokiConfig, prefix+".loki", flags)
}

func (r *RuntimeConfig) AddFlagsWithPrefix(prefix string, flags *pflag.FlagSet) {
	r.Notification.AddFlagsWithPrefix(prefix+".notification", flags)
}

func (r *RuntimeConfig) AddFlags(flags *pflag.FlagSet) {
	r.AddFlagsWithPrefix("alerting.historian", flags)
}

type urlVar struct {
	u **url.URL
}

// String implements flag.Value
func (v urlVar) String() string {
	if v.u == nil || *v.u == nil {
		return ""
	}
	return (*v.u).Redacted()
}

// Set implements flag.Value
func (v urlVar) Set(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	*v.u = u
	return nil
}

// Type implements flag.Value
func (v urlVar) Type() string {
	return "url"
}

func addLokiFlags(l *lokiclient.LokiConfig, prefix string, flags *pflag.FlagSet) {
	flags.Var(urlVar{&l.ReadPathURL}, prefix+".read-url", "URL to Loki instance for performing queries")
	flags.StringVar(&l.BasicAuthUser, prefix+".user", "", "Basic auth Username to authenticate to the Loki instance")
	flags.StringVar(&l.BasicAuthPassword, prefix+".password", "", "Basic auth password to authenticate to the Loki instance")
	flags.StringVar(&l.TenantID, prefix+".tenant-id", "", "Value to use for X-Scope-OrgID")
	flags.DurationVar(&l.MaxQueryLength, prefix+".max-query-length", lokiDefaultMaxQueryLength, "Maximum allowed time range for queries")
	flags.IntVar(&l.MaxQuerySize, prefix+".max-query-size", lokiDefaultMaxQuerySize, "Maximum allowed size of a query string passed to Loki")
}
