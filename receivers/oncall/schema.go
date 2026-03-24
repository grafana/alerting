package oncall

import (
	v1 "github.com/grafana/alerting/receivers/oncall/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type = schema.OnCallType

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Grafana IRM",
	"Grafana IRM settings",
	"Sends alerts to Grafana IRM",
	"", // info
	false, // deprecated
	v1.Schema,
)
