package webex

import (
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/webex/v0mimir1"
	v1 "github.com/grafana/alerting/receivers/webex/v1"
)

const Type schema.IntegrationType = "webex"

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Cisco Webex Teams",
	"Webex settings",
	"Sends notifications to Cisco Webex Teams",
	"Notifications can be configured for any Cisco Webex Teams", // info
	false, // deprecated
	v1.Schema,
	v0mimir1.Schema,
)
