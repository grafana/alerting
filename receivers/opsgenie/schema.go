package opsgenie

import (
	v1 "github.com/grafana/alerting/receivers/opsgenie/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "opsgenie"

func Schema() schema.IntegrationTypeSchema {
	return schema.IntegrationTypeSchema{
		Type:           Type,
		Name:           "OpsGenie",
		Description:    "Sends notifications to OpsGenie",
		Heading:        "OpsGenie settings",
		CurrentVersion: v1.Version,
		Versions: []schema.IntegrationSchemaVersion{
			v1.Schema(),
		},
	}
}
