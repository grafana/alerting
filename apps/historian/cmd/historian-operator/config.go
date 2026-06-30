package main

import (
	"github.com/grafana/grafana-app-sdk/operator"
	"github.com/spf13/pflag"

	historianconfig "github.com/grafana/alerting/apps/historian/pkg/app/config"
)

// config is the top-level configuration for the historian operator, covering
// the webhook server, the metrics server, and the historian app itself.
type config struct {
	Webhook webhookConfig
	Metrics metricsConfig
	App     historianconfig.RuntimeConfig
}

func (c *config) AddFlags(flags *pflag.FlagSet) {
	c.Webhook.AddFlags(flags)
	c.Metrics.AddFlags(flags)
	c.App.AddFlags(flags)
}

type webhookConfig struct {
	operator.RunnerWebhookConfig
}

func (c *webhookConfig) AddFlags(flags *pflag.FlagSet) {
	flags.IntVar(&c.Port, "webhook.port", 8443, "Port on which to serve the webhook and custom route server")
	flags.StringVar(&c.TLSConfig.CertPath, "webhook.tls.cert-path", "", "Path to the TLS certificate to use for the webhook server")
	flags.StringVar(&c.TLSConfig.KeyPath, "webhook.tls.key-path", "", "Path to the TLS private key to use for the webhook server")
}

type metricsConfig struct {
	operator.RunnerMetricsConfig
}

func (c *metricsConfig) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&c.Enabled, "metrics.enabled", true, "Enable the Prometheus metrics server")
	flags.IntVar(&c.Port, "metrics.port", 0, "Port on which to serve Prometheus metrics (0 uses the default)")
}
