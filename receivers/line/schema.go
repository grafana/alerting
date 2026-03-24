package line

import (
	v1 "github.com/grafana/alerting/receivers/line/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type = schema.LineType

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"LINE",
	"LINE notify settings",
	"Send notifications to LINE notify. This integration is deprecated and will be removed in a future release.",
	"", // info
	true, // deprecated
	v1.Schema,
)
