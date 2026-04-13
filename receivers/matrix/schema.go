package matrix

import (
	v1 "github.com/grafana/alerting/receivers/matrix/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type = schema.MatrixType

var Schema = schema.InitSchema(
	schema.IntegrationTypeSchema{
		Type:           Type,
		Name:           "Matrix",
		Description:    "Sends notifications to a Matrix room via the Client-Server API",
		Heading:        "Matrix settings",
		CurrentVersion: v1.Version,
	},
	v1.Schema,
)
