package execute

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	kitlog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alerting/testing/alerting-gen/pkg/config"
	gen "github.com/grafana/alerting/testing/alerting-gen/pkg/generate"
	api "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/client/provisioning"
	"github.com/grafana/grafana-openapi-client-go/models"
)

func Run(cfg config.Config, debug bool) ([]*models.AlertRuleGroup, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Initialize logger based on debug flag using go-kit/log.
	baseLogger := kitlog.NewLogfmtLogger(os.Stderr)
	// Level filter.
	var filtered kitlog.Logger
	if debug {
		filtered = level.NewFilter(baseLogger, level.AllowDebug())
	} else {
		filtered = level.NewFilter(baseLogger, level.AllowInfo())
	}
	logger := kitlog.With(filtered, "ts", kitlog.DefaultTimestampUTC, "caller", kitlog.DefaultCaller)

	// Basic validation.
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

	// Handle --nuke flag.
	if cfg.Nuke {
		if cfg.GrafanaURL == "" {
			return nil, fmt.Errorf("grafana-url is required when using --nuke")
		}
		level.Info(logger).Log("msg", "Nuking all alerting-gen created folders")
		if err := nukeFolders(cfg, logger); err != nil {
			return nil, fmt.Errorf("failed to nuke folders: %w", err)
		}
		level.Info(logger).Log("msg", "Nuke completed successfully")

		// If no generation work to do, exit early.
		if cfg.NumAlerting == 0 && cfg.NumRecording == 0 {
			return nil, nil
		}
	}

	// Default write-ds to query-ds if not explicitly provided.
	if cfg.WriteDS == "" && cfg.QueryDS != "" {
		cfg.WriteDS = cfg.QueryDS
		level.Debug(logger).Log("msg", "Using same data source for write-ds", "uid", cfg.WriteDS)
	}

	// If num-folders is set, create folders dynamically.
	if cfg.NumFolders > 0 {
		if cfg.GrafanaURL == "" {
			return nil, fmt.Errorf("grafana-url is required when num-folders is set (folders need to be created via API)")
		}
		level.Info(logger).Log("msg", "creating folders", "count", cfg.NumFolders)
		createdUIDs, err := createFolders(cfg, cfg.NumFolders, cfg.Seed, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create folders: %w", err)
		}
		cfg.FolderUIDs = createdUIDs
		level.Info(logger).Log("msg", "folders created successfully", "count", len(cfg.FolderUIDs))
	}

	groups, err := gen.GenerateGroups(gen.Config{
		NumAlerting:     cfg.NumAlerting,
		NumRecording:    cfg.NumRecording,
		QueryDS:         cfg.QueryDS,
		WriteDS:         cfg.WriteDS,
		RulesPerGroup:   cfg.RulesPerGroup,
		GroupsPerFolder: cfg.GroupsPerFolder,
		EvalInterval:    cfg.EvalInterval,
		Seed:            cfg.Seed,
		FolderUIDs:      cfg.FolderUIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate groups: %w", err)
	}

	// If Grafana URL is provided, send via provisioning API as well.
	if cfg.GrafanaURL != "" {
		if err := sendViaProvisioning(cfg, groups, logger); err != nil {
			return groups, fmt.Errorf("failed to send rule group via provisioning: %w", err)
		}
	}
	return groups, nil
}

// sendViaProvisioning maps the generated export groups into provisioned group payloads
// and pushes them to Grafana using the provisioning API.
func sendViaProvisioning(cfg config.Config, groups []*models.AlertRuleGroup, logger kitlog.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	workers := max(cfg.Concurrency, 1)
	wg.Add(workers)
	gCh := make(chan *models.AlertRuleGroup)
	var workerErr error

	for i := range workers {
		go func(i int) {
			level.Debug(logger).Log("msg", "Initializing worker", "number", i)
			defer func() {
				level.Debug(logger).Log("msg", "Terminating worker", "number", i)
				wg.Done()
			}()

			// Build client from URL and auth inputs.
			cli, err := newGrafanaClient(cfg.GrafanaURL, cfg.Username, cfg.Password, cfg.Token, cfg.OrgID)
			if err != nil {
				workerErr = err
				cancel()
				return
			}

			for {
				select {
				case <-ctx.Done():
					return
				case g, ok := <-gCh:
					if !ok {
						return
					}

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
					if _, err = cli.Provisioning.PutAlertRuleGroup(params); err != nil {
						workerErr = err
						cancel()
						return
					}
					level.Debug(logger).Log("msg", "Alert rule group created", "folder", g.FolderUID, "group", g.Title)
				}
			}
		}(i)
	}

	// Producer goroutine.
	go func() {
		defer close(gCh)
		for _, g := range groups {
			select {
			case <-ctx.Done():
				return
			case gCh <- g:
			}
		}
	}()

	// Wait for all workers to finish.
	wg.Wait()

	return workerErr
}

// newGrafanaClient creates a configured Grafana HTTP API client for a given base URL and API key.
func newGrafanaClient(baseURL, username, password, token string, orgID int64) (*api.GrafanaHTTPAPI, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("grafana URL is empty")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse grafana URL: %w", err)
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
	client = client.WithRetries(100, 1*time.Minute)
	return client, nil
}

// nukeFolders deletes all alerting-gen created folders.
func nukeFolders(cfg config.Config, logger kitlog.Logger) error {
	cli, err := newGrafanaClient(cfg.GrafanaURL, cfg.Username, cfg.Password, cfg.Token, cfg.OrgID)
	if err != nil {
		return err
	}

	// List all folders.
	params := folders.NewGetFoldersParams()
	resp, err := cli.Folders.GetFolders(params)
	if err != nil {
		return fmt.Errorf("failed to list folders: %w", err)
	}

	// Filter folders with "Alerts Folder" in title (created by alerting-gen).
	var foldersToDelete []string
	for _, folder := range resp.Payload {
		if strings.Contains(folder.Title, "Alerts Folder") {
			foldersToDelete = append(foldersToDelete, folder.UID)
		}
	}

	if len(foldersToDelete) == 0 {
		level.Info(logger).Log("msg", "No alerting-gen folders found to delete")
		return nil
	}

	level.Info(logger).Log("msg", "Deleting folders", "count", len(foldersToDelete))

	// Delete each folder.
	forceDelete := true
	for _, folderUID := range foldersToDelete {
		level.Debug(logger).Log("msg", "Deleting folder", "uid", folderUID)
		deleteParams := folders.NewDeleteFolderParams().
			WithFolderUID(folderUID).
			WithForceDeleteRules(&forceDelete)

		if _, err := cli.Folders.DeleteFolder(deleteParams); err != nil {
			return fmt.Errorf("failed to delete folder %q: %w", folderUID, err)
		}
		level.Debug(logger).Log("msg", "Folder deleted", "uid", folderUID)
	}

	return nil
}

// createFolders creates N folders in Grafana and returns their UIDs.
func createFolders(cfg config.Config, numFolders int, seed int64, logger kitlog.Logger) ([]string, error) {
	cli, err := newGrafanaClient(cfg.GrafanaURL, cfg.Username, cfg.Password, cfg.Token, cfg.OrgID)
	if err != nil {
		return nil, err
	}

	// Generate random folder UIDs using the same approach as alert UIDs.
	uidGen := gen.RandomUID()
	folderUIDs := make([]string, 0, numFolders)

	for i := range numFolders {
		// Use seed + i to get deterministic but unique UIDs per folder.
		folderUID := uidGen.Example(int(seed) + i)
		folderTitle := fmt.Sprintf("Alerts Folder %d", i+1)

		level.Debug(logger).Log("msg", "Creating folder", "uid", folderUID, "title", folderTitle)

		body := &models.CreateFolderCommand{
			UID:   folderUID,
			Title: folderTitle,
		}

		resp, err := cli.Folders.CreateFolder(body)
		if err != nil {
			return nil, fmt.Errorf("failed to create folder %q: %w", folderUID, err)
		}

		folderUIDs = append(folderUIDs, resp.Payload.UID)
		level.Debug(logger).Log("msg", "Folder created", "uid", resp.Payload.UID)
	}

	return folderUIDs, nil
}
