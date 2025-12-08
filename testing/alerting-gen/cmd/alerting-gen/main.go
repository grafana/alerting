package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/grafana/alerting/testing/alerting-gen/pkg/execute"
)

// Config holds CLI inputs

type CLIOptions struct {
	OutPath string
	Debug   bool
	execute.Config
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func parseFlags() CLIOptions {
	var cfg CLIOptions
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

func run(cfg CLIOptions) error {
	groups, err := execute.Run(cfg.Config, cfg.Debug)
	if err != nil {
		return err
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
