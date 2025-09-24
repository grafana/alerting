package pushover

import (
	v1 "github.com/grafana/alerting/receivers/pushover/v1"
	"github.com/grafana/alerting/receivers/schema"
)

const Type schema.IntegrationType = "pushover"

func Schema() schema.IntegrationTypeSchema {
	return schema.IntegrationTypeSchema{
		Type:           Type,
		Name:           "Pushover",
		Description:    "Sends HTTP POST request to the Pushover API",
		Heading:        "Pushover settings",
		CurrentVersion: v1.Version,
		Versions: []schema.IntegrationSchemaVersion{
			v1.Schema(),
		},
	}
}
