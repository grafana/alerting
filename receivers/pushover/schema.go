package pushover

import (
	"github.com/grafana/alerting/receivers/pushover/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/pushover/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type = schema.PushoverType

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Pushover",
	"Pushover settings",
	"Sends HTTP POST request to the Pushover API",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
