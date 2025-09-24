package telegram

import (
	"github.com/grafana/alerting/receivers/schema"
	v1 "github.com/grafana/alerting/receivers/telegram/v1"
)

const Type schema.IntegrationType = "telegram"

func Schema() schema.IntegrationTypeSchema {
	return schema.IntegrationTypeSchema{
		Type:           Type,
		Name:           "Telegram",
		Description:    "Sends notifications to Telegram",
		Heading:        "Telegram API settings",
		CurrentVersion: v1.Version,
		Versions: []schema.IntegrationSchemaVersion{
			v1.Schema(),
		},
	}
}
