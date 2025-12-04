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
	NumAlerting     int    `json:"alertRuleCount"`
	NumRecording    int    `json:"recordingRuleCount"`
	QueryDS         string `json:"queryDatasourceUID"`
	WriteDS         string `json:"writeDatasourceUID"`
	RulesPerGroup   int    `json:"rulesPerGroup"`
	GroupsPerFolder int    `json:"groupsPerFolder"`
	Seed            int64  `json:"seed"`
	UploadOptions
}

type UploadOptions struct {
	GrafanaURL    string `json:"grafanaURL"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Token         string `json:"token,omitempty"`
	OrgID         int64  `json:"orgID"`
	FolderUIDsCSV string `json:"folderUIDs"`
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
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	level.Info(logger).Log("msg", "generated alert rule groups", "count", len(groups))

	// If Grafana URL is provided, send via provisioning API as well
	if cfg.GrafanaURL != "" {
		if err := sendViaProvisioning(cfg, groups, logger); err != nil {
			return groups, fmt.Errorf("sending via provisioning: %w", err)
		}
		level.Info(logger).Log("msg", "successfully sent alert rule groups via provisioning API", "count", len(groups))
	}
	return groups, nil
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
	if token != "" {
		cfg.APIKey = token
	} else {
		cfg.BasicAuth = url.UserPassword(username, password)
	}
	// Only need OrgID from config; host/basePath/schemes already carried by transport.
	cfg.OrgID = orgID
	cli := api.NewHTTPClientWithConfig(nil, cfg)
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
