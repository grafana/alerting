package pagerduty

import (
	"github.com/grafana/alerting/receivers/pagerduty/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/pagerduty/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type = schema.PagerDutyType

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"PagerDuty",
	"PagerDuty settings",
	"Sends notifications to PagerDuty",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
