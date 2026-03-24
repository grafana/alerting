package webhook

import (
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/webhook/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/webhook/v1"
)

const Type = schema.WebhookType

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Webhook",
	"Webhook settings",
	"Sends HTTP POST request to a URL",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
