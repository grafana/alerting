package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	auth "alerting-gen/pkg/auth"
	gen "alerting-gen/pkg/generate"

	kitlog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	runtimeclient "github.com/go-openapi/runtime/client"
	api "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/client/provisioning"
	models "github.com/grafana/grafana-openapi-client-go/models"
)

// Config holds CLI inputs
type Config struct {
	NumAlerting     int
	NumRecording    int
	QueryDS         string
	WriteDS         string
	RulesPerGroup   int
	GroupsPerFolder int
	Seed            int64
	OutPath         string
	// Sending options
	GrafanaURL    string
	Username      string
	Password      string
	Token         string
	OrgID         int64
	FolderUIDsCSV string
	Debug         bool
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func parseFlags() Config {
	var cfg Config
	flag.IntVar(&cfg.NumAlerting, "alerts", 10, "number of alerting rules to generate")
	flag.IntVar(&cfg.NumRecording, "recordings", 0, "number of recording rules to generate")
	flag.StringVar(&cfg.QueryDS, "query-ds", "__expr__", "datasource UID to query from (e.g., __expr__ or prom UID)")
	flag.StringVar(&cfg.WriteDS, "write-ds", "", "datasource UID to write recording rules to (e.g., prom UID)")
	flag.IntVar(&cfg.RulesPerGroup, "rules-per-group", 5, "number of rules per group")
	flag.IntVar(&cfg.GroupsPerFolder, "groups-per-folder", 2, "number of groups per folder")
	flag.Int64Var(&cfg.Seed, "seed", time.Now().UnixNano(), "seed for deterministic generation")
	flag.StringVar(&cfg.OutPath, "out", "", "output file path (defaults to stdout)")
	flag.StringVar(&cfg.GrafanaURL, "grafana-url", "", "Grafana base URL (when set, will send generated rules via provisioning API)")
	flag.StringVar(&cfg.Username, "username", "admin", "Grafana Admin username")
	flag.StringVar(&cfg.Password, "password", "admin", "Grafana Admin password")
	flag.StringVar(&cfg.Token, "token", "", "Grafana service account token (alternative to username/password; takes precedence if set)")
	flag.Int64Var(&cfg.OrgID, "org-id", 1, "Grafana organization ID (optional; API keys are org-scoped)")
	flag.StringVar(&cfg.FolderUIDsCSV, "folder-uids", "default", "Comma-separated list of folder UIDs to distribute groups across (defaults to 'general')")
	flag.BoolVar(&cfg.Debug, "debug", false, "enable debug logging")
	flag.Parse()
	return cfg
}

func run(cfg Config) error {
	// Initialize logger based on debug flag using go-kit/log
	baseLogger := kitlog.NewLogfmtLogger(os.Stderr)
	// Level filter
	var filtered kitlog.Logger
	if cfg.Debug {
		filtered = level.NewFilter(baseLogger, level.AllowDebug())
	} else {
		filtered = level.NewFilter(baseLogger, level.AllowInfo())
	}
	logger := kitlog.With(filtered, "ts", kitlog.DefaultTimestampUTC, "caller", kitlog.DefaultCaller)

	// Basic validation
	if cfg.NumRecording > 0 && cfg.WriteDS == "" {
		return fmt.Errorf("write-ds is required when generating recording rules (set -write-ds to a Prometheus datasource UID)")
	}
	if cfg.GrafanaURL != "" {
		// If token provided, it takes precedence; else require username/password.
		if cfg.Token == "" {
			if cfg.Username == "" {
				return fmt.Errorf("username is required when grafana-url is set (or provide -token instead)")
			}
			if cfg.Password == "" {
				return fmt.Errorf("password is required when grafana-url is set (or provide -token instead)")
			}
		}
	}
	folderUIDs := parseCSV(cfg.FolderUIDsCSV)
	groups, err := gen.GenerateGroups(gen.Config{
		NumAlerting:     cfg.NumAlerting,
		NumRecording:    cfg.NumRecording,
		QueryDS:         cfg.QueryDS,
		WriteDS:         cfg.WriteDS,
		RulesPerGroup:   cfg.RulesPerGroup,
		GroupsPerFolder: cfg.GroupsPerFolder,
		Seed:            cfg.Seed,
		FolderUIDs:      folderUIDs,
	})
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	// If Grafana URL is provided, send via provisioning API as well
	if cfg.GrafanaURL != "" {
		if err := sendViaProvisioning(cfg, groups, logger); err != nil {
			return fmt.Errorf("sending via provisioning: %w", err)
		}
	}

	// Print the same models we would send
	b, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return err
	}
	if cfg.OutPath == "" {
		fmt.Println(string(b))
		fmt.Fprintf(os.Stderr, "seed=%d\n", cfg.Seed)
		return nil
	}
	return os.WriteFile(cfg.OutPath, b, 0o644)
}

// sendViaProvisioning maps the generated export groups into provisioned group payloads
// and pushes them to Grafana using the provisioning API.
func sendViaProvisioning(cfg Config, groups []*models.AlertRuleGroup, logger kitlog.Logger) error {
	// Build client from URL and auth inputs
	cli, err := newGrafanaClient(cfg.GrafanaURL, cfg.Username, cfg.Password, cfg.Token, cfg.OrgID)
	if err != nil {
		return err
	}

	level.Info(logger).Log("msg", "sending alert rule groups", "count", len(groups), "url", cfg.GrafanaURL, "orgID", cfg.OrgID, "user", cfg.Username)

	for _, g := range groups {
		level.Debug(logger).Log("msg", "PUT alert rule group", "folder", g.FolderUID, "group", g.Title, "rules", len(g.Rules))
		body := &models.AlertRuleGroup{
			FolderUID: g.FolderUID,
			Interval:  g.Interval,
			Rules:     g.Rules,
			Title:     g.Title,
		}
		params := provisioning.NewPutAlertRuleGroupParams().
			WithFolderUID(g.FolderUID).
			WithGroup(g.Title).
			WithBody(body)
		if _, err := cli.Provisioning.PutAlertRuleGroup(params); err != nil {
			return fmt.Errorf("put rule group %q in folder %q: %w", g.Title, g.FolderUID, err)
		}
		level.Debug(logger).Log("msg", "PUT alert rule group OK", "folder", g.FolderUID, "group", g.Title)
	}
	return nil
}

// newGrafanaClient creates a configured Grafana HTTP API client for a given base URL and API key.
func newGrafanaClient(baseURL, username, password, token string, orgID int64) (*api.GrafanaHTTPAPI, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("grafana URL is empty")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid grafana URL: %w", err)
	}
	// Prepare transport config once (host/base path/schemes)
	cfg := api.DefaultTransportConfig().
		WithHost(u.Host).
		WithBasePath("/api").
		WithSchemes([]string{u.Scheme})
	cfg.OrgID = orgID
	// Build auth RoundTripper chain (bearer preferred over basic)
	var authRT http.RoundTripper
	if token != "" {
		authRT = auth.NewBearerTokenRoundTripper(token, nil)
	} else {
		authRT = auth.NewBasicAuthRoundTripper(username, password, nil)
	}
	// Custom HTTP client with our auth RoundTripper.
	hc := &http.Client{Transport: authRT, Timeout: 30 * time.Second}
	host := u.Host
	basePath := "/api"
	schemes := []string{u.Scheme}
	transport := runtimeclient.NewWithClient(host, basePath, schemes, hc)
	// Only need OrgID from config; host/basePath/schemes already carried by transport.
	cfg.OrgID = orgID
	cli := api.New(transport, cfg, nil)
	cli = cli.WithRetries(2, 2*time.Second)
	return cli, nil
}

func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
