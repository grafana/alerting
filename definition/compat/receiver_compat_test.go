package compat_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alerting/definition/compat"
	"github.com/grafana/alerting/notify/notifytest"
	jira_v0mimir1 "github.com/grafana/alerting/receivers/jira/v0mimir1"
	pushover_v0mimir1 "github.com/grafana/alerting/receivers/pushover/v0mimir1"
)

func TestRoundTrip_DefinitionToUpstreamToDefinition(t *testing.T) {
	original, err := notifytest.GetMimirReceiverWithAllIntegrations()
	require.NoError(t, err)

	upstream := compat.DefinitionReceiverToUpstreamReceiver(original)
	roundTripped := compat.UpstreamReceiverToDefinitionReceiver(upstream)

	require.Equal(t, original, roundTripped)
}

func TestPushoverCompatPreservesFormattingAndRedactsCredentials(t *testing.T) {
	testCases := []struct {
		name      string
		html      bool
		monospace bool
	}{
		{name: "HTML", html: true, monospace: false},
		{name: "monospace", html: false, monospace: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := pushover_v0mimir1.GetFullValidConfig()
			original.HTML = tc.html
			original.Monospace = tc.monospace

			upstream := compat.DefinitionReceiverToUpstreamReceiver(compat.Receiver{PushoverConfigs: []*pushover_v0mimir1.Config{&original}})
			require.Len(t, upstream.PushoverConfigs, 1)
			cfg := upstream.PushoverConfigs[0]
			require.Equal(t, tc.html, cfg.HTML)
			require.Equal(t, tc.monospace, cfg.Monospace)

			roundTripped := compat.UpstreamReceiverToDefinitionReceiver(upstream)
			require.Len(t, roundTripped.PushoverConfigs, 1)
			require.Equal(t, original, *roundTripped.PushoverConfigs[0])

			for _, config := range []any{cfg, roundTripped.PushoverConfigs[0]} {
				for _, marshal := range []func(any) ([]byte, error){json.Marshal, yaml.Marshal} {
					encoded, err := marshal(config)
					require.NoError(t, err)
					require.NotContains(t, string(encoded), string(original.UserKey))
					require.NotContains(t, string(encoded), string(original.Token))
				}
			}
		})
	}
}

func TestJiraCompatPreservesFieldConfig(t *testing.T) {
	testCases := []struct {
		name string
		v    *bool
	}{
		{name: "nil"},
		{name: "false", v: new(false)},
		{name: "true", v: new(true)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := jira_v0mimir1.GetFullValidConfig()
			original.APIType = "datacenter"
			original.Summary.EnableUpdate = tc.v
			original.Description.EnableUpdate = tc.v

			upstream := compat.DefinitionReceiverToUpstreamReceiver(compat.Receiver{JiraConfigs: []*jira_v0mimir1.Config{&original}})
			roundTripped := compat.UpstreamReceiverToDefinitionReceiver(upstream)
			require.Equal(t, original, *roundTripped.JiraConfigs[0])
		})
	}
}
