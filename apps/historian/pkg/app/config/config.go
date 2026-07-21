package config

import (
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/pflag"

	"github.com/grafana/alerting/notify/historian/lokiclient"
	authtypes "github.com/grafana/authlib/types"
	"github.com/grafana/grafana-app-sdk/simple"
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
	// the caller may see. Results are filtered to those folders. This works in
	// multi-tenant deployments because both the folder API and the authz service
	// are multi-tenant, unlike the single-tenant rules API.
	RBACEnabled bool
	// AccessClient authorizes alert.rules:read per folder for RBAC filtering. It is
	// set programmatically by the deployment wiring (not via a flag). When
	// RBACEnabled is true it must be non-nil, otherwise notification queries fail
	// closed.
	AccessClient authtypes.AccessClient
}

type RuntimeConfig struct {
	GetAlertStateHistoryHandler simple.AppCustomRouteHandler
	Notification                NotificationConfig
}

func (n *NotificationConfig) AddFlagsWithPrefix(prefix string, flags *pflag.FlagSet) {
	flags.BoolVar(&n.Enabled, prefix+".enabled", false, "Enable notification query endpoints")
	flags.BoolVar(&n.RBACEnabled, prefix+".rbac-enabled", false, "Restrict notification history to the alert rules the requesting user can access")
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
