package alertmanager

import (
	"github.com/grafana/alerting/receivers"
	v1 "github.com/grafana/alerting/receivers/alertmanager/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type = schema.AlertManagerType

var Schema = schema.InitSchema(
	schema.IntegrationTypeSchema{
		Type:           Type,
		Name:           "Alertmanager",
		Description:    "Sends notifications to Alertmanager",
		Heading:        "Alertmanager Settings",
		CurrentVersion: v1.Version,
	},
	v1.Schema,
)

var Manifest = receivers.NewManifest(
	Schema,
	v1.Factory,
)
