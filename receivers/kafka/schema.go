package kafka

import (
	v1 "github.com/grafana/alerting/receivers/kafka/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "kafka"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Kafka REST Proxy",
	"Kafka settings",
	"Sends notifications to Kafka Rest Proxy",
	"", // info
	false, // deprecated
	v1.Schema,
)
