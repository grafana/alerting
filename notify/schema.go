package notify

import (
	"github.com/grafana/alerting/receivers/alertmanager"
	"github.com/grafana/alerting/receivers/dingding"
	"github.com/grafana/alerting/receivers/discord"
	"github.com/grafana/alerting/receivers/email"
	"github.com/grafana/alerting/receivers/googlechat"
	"github.com/grafana/alerting/receivers/schema"
)

// GetSchemaForAllIntegrations returns the current schema for all integrations.
func GetSchemaForAllIntegrations() []schema.IntegrationTypeSchema {
	return []schema.IntegrationTypeSchema{
		alertmanager.Schema(),
		dingding.Schema(),
		discord.Schema(),
		email.Schema(),
		googlechat.Schema(),
	}
}
