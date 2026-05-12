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
	webhookV0 "github.com/grafana/alerting/receivers/webhook/v0mimir1"
)

func TestPostableAPIReceiverToAPIReceiver(t *testing.T) {
	t.Run("returns empty when no receivers", func(t *testing.T) {
		r := &definition.PostableApiReceiver{
			Receiver: definition.Receiver{
				Name: "test-receiver",
			},
		}
		actual, err := PostableAPIReceiverToReceiverConfig(r)
		require.NoError(t, err)
		require.Empty(t, actual.Integrations)
		require.Equal(t, r.Name, actual.Name)
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
		actual, err := PostableAPIReceiverToReceiverConfig(r)
		require.NoError(t, err)
		require.Len(t, actual.Integrations, 2)
		require.Equal(t, r.Name, actual.Name)
		expected0, err := PostableGrafanaReceiverToIntegrationConfig(r.GrafanaManagedReceivers[0])
		require.NoError(t, err)
		expected1, err := PostableGrafanaReceiverToIntegrationConfig(r.GrafanaManagedReceivers[1])
		require.NoError(t, err)
		require.Equal(t, *expected0, *actual.Integrations[0])
		require.Equal(t, *expected1, *actual.Integrations[1])
	})
	t.Run("returns error for unknown integration type", func(t *testing.T) {
		r := &definition.PostableApiReceiver{
			Receiver: definition.Receiver{Name: "test-receiver"},
			PostableGrafanaReceivers: definition.PostableGrafanaReceivers{
				GrafanaManagedReceivers: []*definition.PostableGrafanaReceiver{
					{UID: "uid", Name: "name", Type: "unknown_type"},
				},
			},
		}
		_, err := PostableAPIReceiverToReceiverConfig(r)
		require.ErrorContains(t, err, "invalid integration at index 0")
	})
	t.Run("returns error for invalid version", func(t *testing.T) {
		r := &definition.PostableApiReceiver{
			Receiver: definition.Receiver{Name: "test-receiver"},
			PostableGrafanaReceivers: definition.PostableGrafanaReceivers{
				GrafanaManagedReceivers: []*definition.PostableGrafanaReceiver{
					{UID: "uid", Name: "name", Type: "slack", Version: "v99"},
				},
			},
		}
		_, err := PostableAPIReceiverToReceiverConfig(r)
		require.ErrorContains(t, err, "invalid integration at index 0")
		require.ErrorContains(t, err, "invalid version v99")
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
	actual, err := PostableGrafanaReceiverToIntegrationConfig(r)
	require.NoError(t, err)
	require.Equal(t, models.IntegrationConfig{
		UID:                   "test-uid",
		Name:                  "test-name",
		Type:                  schema.SlackType,
		Version:               schema.V1,
		DisableResolveMessage: false,
		Settings:              json.RawMessage(`{ "data" : "test" }`),
		SecureSettings: map[string]string{
			"test": "data",
		},
	}, *actual)

	t.Run("normalizes type casing", func(t *testing.T) {
		r := &definition.PostableGrafanaReceiver{
			UID:  "test-uid",
			Name: "test-name",
			Type: "SLACK",
		}
		actual, err := PostableGrafanaReceiverToIntegrationConfig(r)
		require.NoError(t, err)
		require.Equal(t, schema.SlackType, actual.Type)
	})
}

func TestPostableApiAlertingConfigToApiReceivers(t *testing.T) {
	t.Run("returns empty when no receivers", func(t *testing.T) {
		actual, err := PostableAPIReceiversToReceiverConfigs(nil)
		require.Empty(t, actual)
		require.NoError(t, err)
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
	actual, err := PostableAPIReceiversToReceiverConfigs(receivers)
	require.NoError(t, err)

	t.Run("returns error when a receiver has invalid integration", func(t *testing.T) {
		invalid := []*definition.PostableApiReceiver{
			{
				Receiver: definition.Receiver{Name: "bad-receiver"},
				PostableGrafanaReceivers: definition.PostableGrafanaReceivers{
					GrafanaManagedReceivers: []*definition.PostableGrafanaReceiver{
						{UID: "uid", Name: "name", Type: "unknown_type"},
					},
				},
			},
		}
		_, err := PostableAPIReceiversToReceiverConfigs(invalid)
		require.ErrorContains(t, err, "invalid receiver bad-receiver")
	})

	require.Len(t, actual, 2)
	expected0, err := PostableAPIReceiverToReceiverConfig(receivers[0])
	require.NoError(t, err)
	expected1, err := PostableAPIReceiverToReceiverConfig(receivers[1])
	require.NoError(t, err)
	require.Equal(t, expected0, actual[0])
	require.Equal(t, expected1, actual[1])
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

func TestPostableMimirReceiverToPostableGrafanaReceiver(t *testing.T) {
	t.Run("returns original pointer when receiver has only Grafana integrations", func(t *testing.T) {
		receiver := &definition.PostableApiReceiver{
			Receiver: definition.Receiver{Name: "test"},
			PostableGrafanaReceivers: definition.PostableGrafanaReceivers{
				GrafanaManagedReceivers: []*definition.PostableGrafanaReceiver{
					{UID: "grafana-uid", Name: "test", Type: "email"},
				},
			},
		}
		result, err := PostableMimirReceiverToPostableGrafanaReceiver(receiver)
		require.NoError(t, err)
		assert.Same(t, receiver, result)
	})

	t.Run("converts Mimir integrations to Grafana integrations", func(t *testing.T) {
		wh := webhookV0.GetFullValidConfig()
		receiver := &definition.PostableApiReceiver{
			Receiver: definition.Receiver{
				Name:           "test-receiver",
				WebhookConfigs: []*webhookV0.Config{&wh},
			},
		}

		mimirConfigs, err := ConfigReceiverToMimirIntegrations(receiver.Receiver)
		require.NoError(t, err)
		require.Len(t, mimirConfigs, 1)
		expectedJSON, err := mimirConfigs[0].ConfigJSON()
		require.NoError(t, err)

		result, err := PostableMimirReceiverToPostableGrafanaReceiver(receiver)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotSame(t, receiver, result)
		require.Len(t, result.GrafanaManagedReceivers, 1)

		converted := result.GrafanaManagedReceivers[0]
		assert.Equal(t, "test-receiver", result.Name)
		assert.Equal(t, "test-receiver", converted.Name)
		assert.Equal(t, mimirIntegrationUID("test-receiver", "webhook", 0), converted.UID)
		assert.JSONEq(t, string(expectedJSON), string(converted.Settings))
		assert.False(t, converted.DisableResolveMessage)
		assert.Nil(t, converted.SecureSettings)
		assert.False(t, result.HasMimirIntegrations())
	})

	t.Run("existing Grafana integrations appear before converted Mimir ones", func(t *testing.T) {
		wh := webhookV0.GetFullValidConfig()
		grafanaRecv := &definition.PostableGrafanaReceiver{
			UID:  "existing-uid",
			Name: "existing",
			Type: "email",
		}
		receiver := &definition.PostableApiReceiver{
			Receiver: definition.Receiver{
				Name:           "mixed-receiver",
				WebhookConfigs: []*webhookV0.Config{&wh},
			},
			PostableGrafanaReceivers: definition.PostableGrafanaReceivers{
				GrafanaManagedReceivers: []*definition.PostableGrafanaReceiver{grafanaRecv},
			},
		}

		result, err := PostableMimirReceiverToPostableGrafanaReceiver(receiver)
		require.NoError(t, err)
		require.Len(t, result.GrafanaManagedReceivers, 2)
		assert.Same(t, grafanaRecv, result.GrafanaManagedReceivers[0])
		assert.Equal(t, "webhook", result.GrafanaManagedReceivers[1].Type)
	})

	t.Run("assigns per-type UIDs to converted Mimir integrations", func(t *testing.T) {
		// UIDs are indexed per integration type, so each type starts at 0.
		em := email_v0mimir1.GetFullValidConfig()
		wh := webhookV0.GetFullValidConfig()
		receiver := &definition.PostableApiReceiver{
			Receiver: definition.Receiver{
				Name:           "multi-receiver",
				EmailConfigs:   []*email_v0mimir1.Config{&em},
				WebhookConfigs: []*webhookV0.Config{&wh},
			},
		}

		result, err := PostableMimirReceiverToPostableGrafanaReceiver(receiver)
		require.NoError(t, err)
		require.Len(t, result.GrafanaManagedReceivers, 2)

		assert.Equal(t, mimirIntegrationUID("multi-receiver", "email", 0), result.GrafanaManagedReceivers[0].UID)
		assert.Equal(t, mimirIntegrationUID("multi-receiver", "webhook", 0), result.GrafanaManagedReceivers[1].UID)
	})
}
