package notifytest

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	commoncfg "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/definition"
)

func TestConfigIdempotency(t *testing.T) {
	for iType := range AllValidMimirConfigs {
		t.Run(iType.PkgPath(), func(t *testing.T) {
			cfg, err := GetRawConfigForMimirIntegration(iType, WithDefault)
			require.NoError(t, err)
			cfgInstance, err := GetMimirIntegrationForType(iType, WithDefault)
			require.NoError(t, err)
			data, err := definition.MarshalJSONWithSecrets(cfgInstance)
			require.NoError(t, err)
			require.JSONEq(t, cfg, string(data))
		})
	}
}

func TestHttpConfigIdempotency(t *testing.T) {
	type testCase struct {
		HTTPConfig *commoncfg.HTTPClientConfig `json:"http_config,omitempty"`
	}
	for _, opts := range slices.Sorted(maps.Keys(ValidMimirHTTPConfigs)) {
		if opts == WithLegacyBearerTokenAuth || // Skip because it's a legacy format and is mapped to "authorization"
			opts == WithHeaders { // Skip because handling of headers in JSON is inconsistent unmarshalling is different from marshaling
			continue
		}
		t.Run(string(opts), func(t *testing.T) {
			expected := ValidMimirHTTPConfigs[opts]
			var f testCase
			err := json.Unmarshal([]byte(expected), &f)
			require.NoError(t, err)
			data, err := definition.MarshalJSONWithSecrets(f)
			require.NoError(t, err)
			require.JSONEq(t, expected, string(data))
		})
	}
}
