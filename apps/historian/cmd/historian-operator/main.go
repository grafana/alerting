package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/grafana/grafana-app-sdk/logging"
	"github.com/grafana/grafana-app-sdk/operator"
	"github.com/grafana/grafana-app-sdk/simple"
	"github.com/spf13/pflag"

	historianapis "github.com/grafana/alerting/apps/historian/pkg/apis"
	historianapp "github.com/grafana/alerting/apps/historian/pkg/app"
)

func main() {
	if err := Main(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Main(args []string) error {
	// Configure the default logger to use slog.
	logging.DefaultLogger = logging.NewSLogLogger(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	var cfg config

	flags := pflag.NewFlagSet("historian-operator", pflag.ContinueOnError)
	cfg.AddFlags(flags)

	if err := flags.Parse(args); err != nil {
		return err
	}

	operatorConfig := operator.RunnerConfig{
		WebhookConfig: cfg.Webhook.RunnerWebhookConfig,
		MetricsConfig: cfg.Metrics.RunnerMetricsConfig,
	}

	runner, err := operator.NewRunner(operatorConfig)
	if err != nil {
		return fmt.Errorf("failed to create operator runner: %w", err)
	}

	// Context and cancel for the operator's Run method.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	provider := simple.NewAppProvider(historianapis.LocalManifest(), cfg.App, historianapp.New)

	manifest := provider.Manifest().ManifestData
	logging.DefaultLogger.Info("Starting operator for app",
		"name", manifest.AppName,
		"group", manifest.Group)

	if err := runner.Run(ctx, provider); err != nil {
		return fmt.Errorf("operator exited with error: %w", err)
	}

	logging.DefaultLogger.Info("Normal operator exit")
	return nil
}
