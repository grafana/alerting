package googlechat

import (
	v1 "github.com/grafana/alerting/receivers/googlechat/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "googlechat"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Google Chat",
	"Google Chat settings",
	"Sends notifications to Google Chat via webhooks based on the official JSON message format",
	"", // info
	false, // deprecated
	v1.Schema,
)
