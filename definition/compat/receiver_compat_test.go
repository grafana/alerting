package compat_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/definition/compat"
	"github.com/grafana/alerting/notify/notifytest"
)

func TestRoundTrip_DefinitionToUpstreamToDefinition(t *testing.T) {
	original, err := notifytest.GetMimirReceiverWithAllIntegrations()
	require.NoError(t, err)

	upstream := compat.DefinitionReceiverToUpstreamReceiver(original)
	roundTripped := compat.UpstreamReceiverToDefinitionReceiver(upstream)

	require.Equal(t, original, roundTripped)
}
