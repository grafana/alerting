package threema

import (
	"github.com/grafana/alerting/receivers/schema"
	v1 "github.com/grafana/alerting/receivers/threema/v1"
)

const Type = schema.ThreemaType

var Schema = schema.NewIntegrationTypeSchema(
	Type,
	v1.Version, // currentVersion
	"Threema Gateway",
	"Threema Gateway settings",
	"Sends notifications to Threema using Threema Gateway (Basic IDs)",
	"Notifications can be configured for any Threema Gateway ID of type \"Basic\". End-to-End IDs are not currently supported.The Threema Gateway ID can be set up at https://gateway.threema.ch/.", // info
	false, // deprecated
	v1.Schema,
)
