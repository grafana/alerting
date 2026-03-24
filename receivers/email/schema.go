package email

import (
	"github.com/grafana/alerting/receivers/email/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/email/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "email"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Email",
	"Email settings",
	"Sends notifications using Grafana server configured SMTP settings",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
