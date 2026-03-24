package alertmanager

import (
	v1 "github.com/grafana/alerting/receivers/alertmanager/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "prometheus-alertmanager"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Alertmanager",
	"Alertmanager Settings",
	"Sends notifications to Alertmanager",
	"", // info
	false, // deprecated
	v1.Schema,
)
