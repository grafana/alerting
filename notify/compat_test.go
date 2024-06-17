package notify

import (
	"encoding/json"
	"testing"

	"github.com/grafana/alerting/definition"
	"github.com/prometheus/alertmanager/config"
	"github.com/stretchr/testify/require"
)

func TestPostableApiReceiverToApiReceiver(t *testing.T) {
	postableReceiver := &definition.PostableApiReceiver{
		Receiver: config.Receiver{
			Name: "test",
		},
		PostableGrafanaReceivers: definition.PostableGrafanaReceivers{
			GrafanaManagedReceivers: []*definition.PostableGrafanaReceiver{{
				UID:                   "abc",
				Name:                  "test",
				Type:                  "slack",
				DisableResolveMessage: true,
				Settings:              definition.RawMessage{'b', 'y', 't', 'e', 's'},
				SecureSettings:        map[string]string{"key": "value"},
			}},
		},
	}
	receiver := PostableAPIReceiverToAPIReceiver(postableReceiver)

	require.Equal(t, "test", receiver.Name)
	require.Equal(t, 1, len(receiver.GrafanaIntegrations.Integrations))

	i := receiver.GrafanaIntegrations.Integrations[0]
	require.Equal(t, "abc", i.UID)
	require.Equal(t, "test", i.Name)
	require.Equal(t, "slack", i.Type)
	require.Equal(t, true, i.DisableResolveMessage)
	require.Equal(t, json.RawMessage{'b', 'y', 't', 'e', 's'}, i.Settings)
	require.Equal(t, map[string]string{"key": "value"}, i.SecureSettings)
}
