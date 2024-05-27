package definition

import (
	"encoding/json"
	"testing"

	"github.com/prometheus/alertmanager/config"
	"github.com/stretchr/testify/require"
)

var validConfig = []byte(`{"route":{"receiver":"grafana-default-email","routes":[{"receiver":"grafana-default-email","object_matchers":[["a","=","b"]],"mute_time_intervals":["test1"]}]},"mute_time_intervals":[{"name":"test1","time_intervals":[{"times":[{"start_time":"00:00","end_time":"12:00"}]}]}],"templates":null,"receivers":[{"name":"grafana-default-email","grafana_managed_receiver_configs":[{"uid":"uxwfZvtnz","name":"email receiver","type":"email","disableResolveMessage":false,"settings":{"addresses":"<example@email.com>"},"secureFields":{}}]}]}`)

func TestGrafanaToUpstreamConfig(t *testing.T) {
	cfg, err := Load(validConfig)
	require.NoError(t, err)
	upstream := GrafanaToUpstreamConfig(cfg)

	require.Equal(t, cfg.Global, upstream.Global)
	require.Equal(t, cfg.Route.AsAMRoute(), upstream.Route)
	require.Equal(t, cfg.InhibitRules, upstream.InhibitRules)
	require.Equal(t, cfg.Templates, upstream.Templates)
	require.Equal(t, cfg.MuteTimeIntervals, upstream.MuteTimeIntervals)
	require.Equal(t, cfg.TimeIntervals, upstream.TimeIntervals)

	for i, r := range cfg.Receivers {
		require.Equal(t, r.Name, upstream.Receivers[i].Name)
	}
}

func TestPostableApiReceiverToApiReceiver(t *testing.T) {
	postableReceiver := &PostableApiReceiver{
		Receiver: config.Receiver{
			Name: "test",
		},
		PostableGrafanaReceivers: PostableGrafanaReceivers{
			GrafanaManagedReceivers: []*PostableGrafanaReceiver{{
				UID:                   "abc",
				Name:                  "test",
				Type:                  "slack",
				DisableResolveMessage: true,
				Settings:              RawMessage{'b', 'y', 't', 'e', 's'},
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
