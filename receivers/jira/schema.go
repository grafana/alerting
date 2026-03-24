package jira

import (
	"github.com/grafana/alerting/receivers/jira/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/jira/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "jira"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Jira",
	"Jira settings",
	"Creates Jira issues from alerts",
	"", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
