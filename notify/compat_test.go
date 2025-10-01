package notify

import (
	"encoding/json"
	"testing"

	"github.com/prometheus/alertmanager/config"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/definition"
	"github.com/grafana/alerting/models"
)

func TestPostableAPIReceiverToAPIReceiver(t *testing.T) {
	t.Run("returns empty when no receivers", func(t *testing.T) {
		r := &definition.PostableApiReceiver{
			Receiver: config.Receiver{
				Name: "test-receiver",
			},
		}
		actual := PostableAPIReceiverToAPIReceiver(r)
		require.Empty(t, actual.Integrations)
		require.Equal(t, r.Receiver, actual.ConfigReceiver)
	})
	t.Run("converts receivers", func(t *testing.T) {
		r := &definition.PostableApiReceiver{
			Receiver: config.Receiver{
				Name: "test-receiver",
			},
			PostableGrafanaReceivers: definition.PostableGrafanaReceivers{
				GrafanaManagedReceivers: []*definition.PostableGrafanaReceiver{
					{
						UID:                   "test-uid",
						Name:                  "test-name",
						Type:                  "slack",
						DisableResolveMessage: false,
						Settings:              definition.RawMessage(`{ "data" : "test" }`),
						SecureSettings: map[string]string{
							"test": "data",
						},
					},
					{
						UID:                   "test-uid2",
						Name:                  "test-name2",
						Type:                  "webhook",
						DisableResolveMessage: false,
						Settings:              definition.RawMessage(`{ "data2" : "test2" }`),
						SecureSettings: map[string]string{
							"test2": "data2",
						},
					},
				},
			},
		}
		actual := PostableAPIReceiverToAPIReceiver(r)
		require.Len(t, actual.Integrations, 2)
		require.Equal(t, r.Receiver, actual.ConfigReceiver)
		require.Equal(t, *PostableGrafanaReceiverToIntegrationConfig(r.GrafanaManagedReceivers[0]), *actual.Integrations[0])
		require.Equal(t, *PostableGrafanaReceiverToIntegrationConfig(r.GrafanaManagedReceivers[1]), *actual.Integrations[1])
	})
}

func TestPostableGrafanaReceiverToGrafanaIntegrationConfig(t *testing.T) {
	r := &definition.PostableGrafanaReceiver{
		UID:                   "test-uid",
		Name:                  "test-name",
		Type:                  "slack",
		DisableResolveMessage: false,
		Settings:              definition.RawMessage(`{ "data" : "test" }`),
		SecureSettings: map[string]string{
			"test": "data",
		},
	}
	actual := PostableGrafanaReceiverToIntegrationConfig(r)
	require.Equal(t, models.IntegrationConfig{
		UID:                   "test-uid",
		Name:                  "test-name",
		Type:                  "slack",
		DisableResolveMessage: false,
		Settings:              json.RawMessage(`{ "data" : "test" }`),
		SecureSettings: map[string]string{
			"test": "data",
		},
	}, *actual)
}

func TestPostableApiAlertingConfigToApiReceivers(t *testing.T) {
	t.Run("returns empty when no receivers", func(t *testing.T) {
		actual := PostableAPIReceiversToAPIReceivers(nil)
		require.Empty(t, actual)
	})
	receivers := []*definition.PostableApiReceiver{
		{
			Receiver: config.Receiver{
				Name: "test-receiver",
			},
			PostableGrafanaReceivers: definition.PostableGrafanaReceivers{
				GrafanaManagedReceivers: []*definition.PostableGrafanaReceiver{
					{
						UID:                   "test-uid",
						Name:                  "test-name",
						Type:                  "slack",
						DisableResolveMessage: false,
						Settings:              definition.RawMessage(`{ "data" : "test" }`),
						SecureSettings: map[string]string{
							"test": "data",
						},
					},
				},
			},
		},
		{
			Receiver: config.Receiver{
				Name: "test-receiver2",
			},
			PostableGrafanaReceivers: definition.PostableGrafanaReceivers{
				GrafanaManagedReceivers: []*definition.PostableGrafanaReceiver{
					{
						UID:                   "test-uid2",
						Name:                  "test-name1",
						Type:                  "slack",
						DisableResolveMessage: false,
						Settings:              definition.RawMessage(`{ "data" : "test" }`),
						SecureSettings: map[string]string{
							"test": "data",
						},
					},
				},
			},
		},
	}
	actual := PostableAPIReceiversToAPIReceivers(receivers)

	require.Len(t, actual, 2)
	require.Equal(t, PostableAPIReceiverToAPIReceiver(receivers[0]), actual[0])
	require.Equal(t, PostableAPIReceiverToAPIReceiver(receivers[1]), actual[1])
}
