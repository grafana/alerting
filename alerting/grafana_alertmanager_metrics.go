package alerting

import (
	"github.com/prometheus/alertmanager/api/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type GrafanaAlertmanagerMetrics struct {
	Registerer prometheus.Registerer
	*metrics.Alerts
}

// NewGrafanaAlertmanagerMetrics creates a set of metrics for the Alertmanager.
func NewGrafanaAlertmanagerMetrics(r prometheus.Registerer) *GrafanaAlertmanagerMetrics {
	return &GrafanaAlertmanagerMetrics{
		Registerer: r,
		Alerts:     metrics.NewAlerts("grafana", r),
	}
}
