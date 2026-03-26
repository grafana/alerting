package kafka

import (
	"github.com/grafana/alerting/receivers"
	v1 "github.com/grafana/alerting/receivers/kafka/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type = schema.KafkaType

var Schema = schema.InitSchema(
	schema.IntegrationTypeSchema{
		Type:           Type,
		Name:           "Kafka REST Proxy",
		Description:    "Sends notifications to Kafka Rest Proxy",
		Heading:        "Kafka settings",
		CurrentVersion: v1.Version,
	},
	v1.Schema,
)

var Manifest = receivers.NewManifest(Schema, v1.Factory)
