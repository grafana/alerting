package compat_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/definition/compat"
	"github.com/grafana/alerting/notify/notifytest"
	jira_v0mimir1 "github.com/grafana/alerting/receivers/jira/v0mimir1"
)

func TestRoundTrip_DefinitionToUpstreamToDefinition(t *testing.T) {
	original, err := notifytest.GetMimirReceiverWithAllIntegrations()
	require.NoError(t, err)

	upstream := compat.DefinitionReceiverToUpstreamReceiver(original)
	roundTripped := compat.UpstreamReceiverToDefinitionReceiver(upstream)

	require.Equal(t, original, roundTripped)
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
