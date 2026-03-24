package teams

import (
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/teams/v0mimir1"
	"github.com/grafana/alerting/receivers/teams/v0mimir2"
	v1 "github.com/grafana/alerting/receivers/teams/v1"
)

const Type schema.IntegrationType = "teams"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Microsoft Teams",
	"Teams settings",
	"Sends notifications using Incoming Webhook connector to Microsoft Teams",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir2.Schema,
	v0mimir1.Schema,
)
