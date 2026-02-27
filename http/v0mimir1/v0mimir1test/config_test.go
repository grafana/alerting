package v0mimir1test

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alerting/definition"
	"github.com/grafana/alerting/http/v0mimir1"
)

func TestHttpConfigIdempotency(t *testing.T) {
	type testCase struct {
		HTTPConfig *v0mimir1.HTTPClientConfig `json:"http_config,omitempty" yaml:"http_config,omitempty"`
	}
	for _, opts := range slices.Sorted(maps.Keys(ValidMimirHTTPConfigs)) {
		if opts == WithLegacyBearerTokenAuth { // Skip because it's a legacy format and is mapped to "authorization"
			continue
		}
		t.Run(string(opts), func(t *testing.T) {
			expected := ValidMimirHTTPConfigs[opts]
			var f testCase
			err := json.Unmarshal([]byte(expected), &f)
			require.NoError(t, err)

			data, err := definition.MarshalJSONWithSecrets(f)
			require.NoError(t, err)
			assert.JSONEq(t, expected, string(data))

			t.Run("unmarshal JSON with YAML", func(t *testing.T) {
				var actual testCase
				err := yaml.Unmarshal([]byte(expected), &actual)
				require.NoError(t, err)
				require.Equal(t, f, actual)
			})
		})
	}
}
