package pagerduty

import (
	v1 "github.com/grafana/alerting/receivers/pagerduty/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "pagerduty"

func Schema() schema.IntegrationTypeSchema {
	return schema.IntegrationTypeSchema{
		Type:           Type,
		Name:           "PagerDuty",
		Description:    "Sends notifications to PagerDuty",
		Heading:        "PagerDuty settings",
		CurrentVersion: v1.Version,
		Versions: []schema.IntegrationSchemaVersion{
			v1.Schema(),
		},
	}
}
