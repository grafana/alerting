package sns

import (
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/sns/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/sns/v1"
)

const Type schema.IntegrationType = "sns"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"AWS SNS",
	"AWS SNS settings",
	"Sends notifications to AWS Simple Notification Service",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
