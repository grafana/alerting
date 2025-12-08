package execute

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	kitlog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	gen "github.com/grafana/alerting/testing/alerting-gen/pkg/generate"
	api "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/client/provisioning"
	"github.com/grafana/grafana-openapi-client-go/models"
)

type Config struct {
	NumAlerting     int
	NumRecording    int
	QueryDS         string
	WriteDS         string
	RulesPerGroup   int
	GroupsPerFolder int
	Seed            int64
	UploadOptions
}

type UploadOptions struct {
	GrafanaURL    string
	Username      string
	Password      string
	Token         string
	OrgID         int64
	FolderUIDsCSV string
}

func Run(cfg Config, debug bool) ([]*models.AlertRuleGroup, error) {
	// Initialize logger based on debug flag using go-kit/log
	baseLogger := kitlog.NewLogfmtLogger(os.Stderr)
	// Level filter
	var filtered kitlog.Logger
	if debug {
		filtered = level.NewFilter(baseLogger, level.AllowDebug())
	} else {
		filtered = level.NewFilter(baseLogger, level.AllowInfo())
	}
	logger := kitlog.With(filtered, "ts", kitlog.DefaultTimestampUTC, "caller", kitlog.DefaultCaller)

	// Basic validation
	if cfg.NumRecording > 0 && cfg.WriteDS == "" {
		return nil, fmt.Errorf("write-ds is required when generating recording rules (set -write-ds to a Prometheus datasource UID)")
	}
	if cfg.GrafanaURL != "" {
		// If token provided, it takes precedence; else require username/password.
		if cfg.Token == "" {
			if cfg.Username == "" {
				return nil, fmt.Errorf("username is required when grafana-url is set (or provide -token instead)")
			}
			if cfg.Password == "" {
				return nil, fmt.Errorf("password is required when grafana-url is set (or provide -token instead)")
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
		return nil, fmt.Errorf("generate: %w", err)
	}

	// If Grafana URL is provided, send via provisioning API as well
	if cfg.GrafanaURL != "" {
		if err := sendViaProvisioning(cfg, groups, logger); err != nil {
			return groups, fmt.Errorf("failed to send rule group via provisioning: %w", err)
		}
	}
	return groups, nil
}

// sendViaProvisioning maps the generated export groups into provisioned group payloads
// and pushes them to Grafana using the provisioning API.
func sendViaProvisioning(cfg Config, groups []*models.AlertRuleGroup, logger kitlog.Logger) error {
	// Build client from URL and auth inputs.
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
			return fmt.Errorf("PUT rule group %q in folder %q: %w", g.Title, g.FolderUID, err)
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

	// Build transport config with authentication.
	cfg := api.DefaultTransportConfig().
		WithHost(u.Host).
		WithBasePath("/api").
		WithSchemes([]string{u.Scheme})

	// Set authentication method.
	if token != "" {
		cfg.APIKey = token
	} else if username != "" && password != "" {
		cfg.BasicAuth = url.UserPassword(username, password)
	}

	// Set org ID (only works with BasicAuth, not APIKey since tokens are org-scoped).
	cfg.OrgID = orgID

	// Create client with retries.
	client := api.NewHTTPClientWithConfig(nil, cfg)
	client = client.WithRetries(2, 2*time.Second)
	return client, nil
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
