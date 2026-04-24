package line

import (
	"github.com/grafana/alerting/receivers"
	v1 "github.com/grafana/alerting/receivers/line/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type = schema.LineType

var Schema = schema.InitSchema(
	schema.IntegrationTypeSchema{
		Type:           Type,
		Name:           "LINE",
		Description:    "Send notifications to LINE notify. This integration is deprecated and will be removed in a future release.",
		Heading:        "LINE notify settings",
		CurrentVersion: v1.Version,
		Deprecated:     true,
	},
	v1.Schema,
)

var Manifest = receivers.NewManifest(Schema, v1.Factory)
