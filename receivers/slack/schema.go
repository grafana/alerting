package slack

import (
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/slack/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/slack/v1"
)

const Type schema.IntegrationType = "slack"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Slack",
	"Slack settings",
	"Sends notifications to Slack",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
