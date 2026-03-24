package victorops

import (
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/victorops/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/victorops/v1"
)

const Type schema.IntegrationType = "victorops"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"VictorOps",
	"VictorOps settings",
	"Sends notifications to VictorOps",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
