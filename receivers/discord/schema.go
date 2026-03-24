package discord

import (
	"github.com/grafana/alerting/receivers/discord/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/discord/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type = schema.DiscordType

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Discord",
	"Discord settings",
	"Sends notifications to Discord",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
