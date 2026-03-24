package opsgenie

import (
	"github.com/grafana/alerting/receivers/opsgenie/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/opsgenie/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "opsgenie"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"OpsGenie",
	"OpsGenie settings",
	"Sends notifications to OpsGenie",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
