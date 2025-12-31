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
	gen "github.com/grafana/alerting/testing/alerting-gen/pkg/generate"
	api "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/client/provisioning"
	"github.com/grafana/grafana-openapi-client-go/models"
)

func Run(cfg Config, debug bool) ([]*models.AlertRuleGroup, error) {
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

	// Handle --nuke flag.
	if cfg.Nuke {
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

	// If num-folders is set and it's not a dry run, create folders dynamically.
	if cfg.NumFolders > 0 && !cfg.DryRun {
		level.Info(logger).Log("msg", "Creating folders", "count", cfg.NumFolders)
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

	// If we need to create resources, use the provisioning API.
	if !cfg.DryRun {
		if err := sendViaProvisioning(cfg, groups, logger); err != nil {
			return groups, fmt.Errorf("failed to send rule group via provisioning: %w", err)
		}
	}

	return groups, nil
}

// sendViaProvisioning maps the generated export groups into provisioned group payloads
// and pushes them to Grafana using the provisioning API.
func sendViaProvisioning(cfg Config, groups []*models.AlertRuleGroup, logger kitlog.Logger) error {
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
func nukeFolders(cfg Config, logger kitlog.Logger) error {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(cfg.Concurrency)
	uidCh := make(chan string)
	errCh := make(chan error, 1)
	forceDelete := true
	for range cfg.Concurrency {
		go func() {
			defer wg.Done()
			for {
				select {
				case uid, ok := <-uidCh:
					if !ok {
						return
					}
					level.Debug(logger).Log("msg", "Deleting folder", "uid", uid)
					deleteParams := folders.NewDeleteFolderParams().
						WithFolderUID(uid).
						WithForceDeleteRules(&forceDelete)

					if _, err := cli.Folders.DeleteFolder(deleteParams); err != nil {
						// Non-blocking send, only first error gets through.
						select {
						case errCh <- fmt.Errorf("failed to delete folder %q: %w", uid, err):
						default:
						}
						cancel()
						return
					}
					level.Debug(logger).Log("msg", "Folder deleted", "uid", uid)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Producer goroutine.
	go func() {
		defer close(uidCh)
		for _, uid := range foldersToDelete {
			select {
			case <-ctx.Done():
				return
			case uidCh <- uid:
			}
		}
	}()

	wg.Wait()

	// Check if we got an error.
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// createFolders creates N folders in Grafana and returns their UIDs.
func createFolders(cfg Config, numFolders int, seed int64, logger kitlog.Logger) ([]string, error) {
	// Generate random folder UIDs using the same approach as alert UIDs.
	uidGen := gen.RandomUID()
	folderUIDs := make([]string, 0, numFolders)
	for i := range numFolders {
		// Use seed + i to get deterministic but unique UIDs per folder.
		folderUID := uidGen.Example(int(seed) + i)
		folderUIDs = append(folderUIDs, folderUID)
	}

	// If it's a dry run, return the folder UIDs.
	if cfg.DryRun {
		return folderUIDs, nil
	}

	cli, err := newGrafanaClient(cfg.GrafanaURL, cfg.Username, cfg.Password, cfg.Token, cfg.OrgID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(cfg.Concurrency)
	uidCh := make(chan string)
	errCh := make(chan error, 1)
	for range cfg.Concurrency {
		go func() {
			defer wg.Done()
			for {
				select {
				case uid, ok := <-uidCh:
					if !ok {
						return
					}
					folderTitle := fmt.Sprintf("Alerts Folder %s", uid)

					level.Debug(logger).Log("msg", "Creating folder", "uid", uid, "title", folderTitle)

					body := &models.CreateFolderCommand{
						UID:   uid,
						Title: folderTitle,
					}

					resp, err := cli.Folders.CreateFolder(body)
					if err != nil {
						// Non-blocking send, only first error gets through.
						select {
						case errCh <- fmt.Errorf("failed to create folder %q: %w", uid, err):
						default:
						}
						cancel()
						return
					}
					level.Debug(logger).Log("msg", "Folder created", "uid", resp.Payload.UID)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Producer goroutine.
	go func() {
		defer close(uidCh)
		for _, uid := range folderUIDs {
			select {
			case <-ctx.Done():
				return
			case uidCh <- uid:
			}
		}
	}()

	wg.Wait()

	// Check if we got an error.
	select {
	case err := <-errCh:
		return folderUIDs, err
	default:
		return folderUIDs, nil
	}
}
