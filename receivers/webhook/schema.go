package webhook

import (
	"github.com/grafana/alerting/receivers/schema"
	v1 "github.com/grafana/alerting/receivers/webhook/v1"
)

const Type schema.IntegrationType = "webhook"

func Schema() schema.IntegrationTypeSchema {
	return schema.IntegrationTypeSchema{
		Type:           Type,
		Name:           "Webhook",
		Description:    "Sends HTTP POST request to a URL",
		Heading:        "Webhook settings",
		CurrentVersion: v1.Version,
		Versions: []schema.IntegrationSchemaVersion{
			v1.Schema(),
		},
	}
}
