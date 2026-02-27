package notifytest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/definition"
	"github.com/grafana/alerting/http/v0mimir1/v0mimir1test"
)

func TestConfigIdempotency(t *testing.T) {
	for iType := range AllValidMimirConfigs {
		t.Run(iType.PkgPath(), func(t *testing.T) {
			cfg, err := GetRawConfigForMimirIntegration(iType, v0mimir1test.WithDefault)
			require.NoError(t, err)
			cfgInstance, err := GetMimirIntegrationForType(iType, v0mimir1test.WithDefault)
			require.NoError(t, err)
			data, err := definition.MarshalJSONWithSecrets(cfgInstance)
			require.NoError(t, err)
			require.JSONEq(t, cfg, string(data))
		})
	}
}
