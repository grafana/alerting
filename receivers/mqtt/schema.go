package mqtt

import (
	v1 "github.com/grafana/alerting/receivers/mqtt/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "mqtt"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"MQTT",
	"MQTT settings",
	"Sends notifications to an MQTT broker",
	"The MQTT notifier sends messages to an MQTT broker. The message is sent to the topic specified in the configuration. ", // info
	false, // deprecated
	v1.Schema,
)
