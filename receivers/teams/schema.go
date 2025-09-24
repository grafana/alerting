package teams

import (
	"github.com/grafana/alerting/receivers/schema"
	v1 "github.com/grafana/alerting/receivers/teams/v1"
)

const Type schema.IntegrationType = "teams"

func Schema() schema.IntegrationTypeSchema {
	return schema.IntegrationTypeSchema{
		Type:           Type,
		Name:           "Microsoft Teams",
		Description:    "Sends notifications using Incoming Webhook connector to Microsoft Teams",
		Heading:        "Teams settings",
		CurrentVersion: v1.Version,
		Versions: []schema.IntegrationSchemaVersion{
			v1.Schema(),
		},
	}
}
