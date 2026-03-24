package telegram

import (
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/telegram/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/telegram/v1"
)

const Type schema.IntegrationType = "telegram"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Telegram",
	"Telegram API settings",
	"Sends notifications to Telegram",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
