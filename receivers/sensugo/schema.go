package sensugo

import (
	"github.com/grafana/alerting/receivers/schema"
	v1 "github.com/grafana/alerting/receivers/sensugo/v1"
)

const Type schema.IntegrationType = "sensugo"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Sensu Go",
	"Sensu Go Settings",
	"Sends HTTP POST request to a Sensu Go API",
	"", // info
	false, // deprecated
	v1.Schema,
)
