package notify

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/definition"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/notifytest"
	"github.com/grafana/alerting/receivers/email"
	email_v0mimir1 "github.com/grafana/alerting/receivers/email/v0mimir1"
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/teams"
	teams_v0mimir1 "github.com/grafana/alerting/receivers/teams/v0mimir1"
	teams_v0mimir2 "github.com/grafana/alerting/receivers/teams/v0mimir2"
)

func TestPostableAPIReceiverToAPIReceiver(t *testing.T) {
	t.Run("returns empty when no receivers", func(t *testing.T) {
		r := &definition.PostableApiReceiver{
			Receiver: definition.Receiver{
				Name: "test-receiver",
			},
		}
		actual := PostableAPIReceiverToAPIReceiver(r)
		require.Empty(t, actual.Integrations)
		require.Equal(t, r.Receiver, actual.ConfigReceiver)
	})
	t.Run("converts receivers", func(t *testing.T) {
		r := &definition.PostableApiReceiver{
			Receiver: definition.Receiver{
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
			Receiver: definition.Receiver{
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
			Receiver: definition.Receiver{
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

func TestConfigReceiverToMimirIntegrations(t *testing.T) {
	r, err := notifytest.GetMimirReceiverWithAllIntegrations()
	require.NoError(t, err)
	actual, err := ConfigReceiverToMimirIntegrations(r)
	require.NoError(t, err)
	require.Len(t, actual, len(notifytest.AllValidMimirConfigs))
	idx := slices.IndexFunc(actual, func(e MimirIntegrationConfig) bool {
		return e.Schema.Type() == email.Type
	})
	require.IsType(t, email_v0mimir1.Config{}, actual[idx].Config)

	t.Run("correctly maps teams versions", func(t *testing.T) {
		found := 0
		for _, ic := range actual {
			if ic.Schema.Type() != teams.Type {
				continue
			}
			switch ic.Schema.Version {
			case schema.V0mimir1:
				found++
				require.IsType(t, teams_v0mimir1.Config{}, ic.Config)
			case schema.V0mimir2:
				found++
				require.IsType(t, teams_v0mimir2.Config{}, ic.Config)
			case schema.V1:
				require.Fail(t, "unexpected V1 version for msteams integration")
			default:
				require.Fail(t, "unexpected version for msteams integration")
			}
		}
		assert.Equal(t, 2, found, "expected 2 teams integrations")
	})
	t.Run("should not fail if empty", func(t *testing.T) {
		actual, err = ConfigReceiverToMimirIntegrations(ConfigReceiver{Name: "empty"})
		require.NoError(t, err)
		require.Empty(t, actual)
	})
}
