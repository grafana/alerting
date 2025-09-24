package mqtt

import (
	v1 "github.com/grafana/alerting/receivers/mqtt/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "mqtt"

func Schema() schema.IntegrationTypeSchema {
	return schema.IntegrationTypeSchema{
		Type:           Type,
		Name:        "MQTT",
		Description: "Sends notifications to an MQTT broker",
		Heading:     "MQTT settings",
		CurrentVersion: v1.Version,
		Versions: []schema.IntegrationSchemaVersion{
			v1.Schema(),
		},
	}
}

