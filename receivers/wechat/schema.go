package wechat

import (
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/wechat/v0mimir1"
)

const Type schema.IntegrationType = "wechat"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v0mimir1.Version, // currentVersion
	"WeChat",
	"WeChat settings",
	"Sends notifications to WeChat",
	"", // info
	false, // deprecated
	v0mimir1.Schema,
)
