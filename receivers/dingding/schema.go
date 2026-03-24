package dingding

import (
	v1 "github.com/grafana/alerting/receivers/dingding/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type = schema.DingDingType

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"DingDing",
	"DingDing settings",
	"Sends HTTP POST request to DingDing",
	"", // info
	false, // deprecated
	v1.Schema,
)
