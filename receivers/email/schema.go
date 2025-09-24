package email

import (
	"github.com/grafana/alerting/receivers/email/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/email/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "email"

func Schema() schema.IntegrationTypeSchema {
	return schema.IntegrationTypeSchema{
		Type:           Type,
		Name:           "Email",
		Heading:        "Email settings",
		Description:    "Send notification over SMTP",
		CurrentVersion: v1.Version,
		Versions: []schema.IntegrationSchemaVersion{
			v1.Schema(),
			v0mimir1.Schema(),
		},
	}
}
