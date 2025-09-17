package notify

import (
	"github.com/grafana/alerting/receivers/alertmanager"
	"github.com/grafana/alerting/receivers/schema"
)

// GetSchemaForAllIntegrations returns the current schema for all integrations.
func GetSchemaForAllIntegrations() []schema.IntegrationTypeSchema {
	return []schema.IntegrationTypeSchema{
		alertmanager.Schema(),
	}
}
