package v1

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestConfigNumericCompatibility(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    int64
		invalid bool
	}{
		{`12`, 12, false}, {`"12"`, 12, false}, {`-1`, -1, false},
		{`null`, 0, false}, {`""`, 0, false},
		{`"bad"`, 0, true}, {`1.5`, 0, true}, {`1e2`, 0, true},
		{`true`, 0, true}, {`[]`, 0, true}, {`{}`, 0, true},
		{`"999999999999999999999999"`, math.MaxInt64, true},
		{`"-999999999999999999999999"`, math.MinInt64, true},
	} {
		for _, key := range []string{"priority", "okPriority", "retry", "expire"} {
			t.Run(key+"/"+tc.value, func(t *testing.T) {
				input := fmt.Sprintf(`{"userKey":"user","apiToken":"token",%q:%s}`, key, tc.value)
				cfg, err := NewConfig(json.RawMessage(input), receiversTesting.DecryptForTesting(nil))
				if tc.invalid && (key == "priority" || key == "okPriority") {
					require.Error(t, err)
					require.Equal(t, "user", cfg.UserKey)
					require.Equal(t, "token", cfg.APIToken)
					require.False(t, cfg.Upload)
				} else {
					require.NoError(t, err)
				}
				switch key {
				case "priority":
					require.Equal(t, tc.want, cfg.AlertingPriority)
				case "okPriority":
					require.Equal(t, tc.want, cfg.OkPriority)
				case "retry":
					require.Equal(t, tc.want, cfg.Retry)
				case "expire":
					require.Equal(t, tc.want, cfg.Expire)
				}
			})
		}
	}
}

func TestConfigUploadCompatibility(t *testing.T) {
	for _, tc := range []struct {
		fields string
		want   bool
	}{
		{"", true}, {`,"uploadImage":null`, true}, {`,"uploadImage":true`, true},
		{`,"uploadImage":false`, false},
		{`,"uploadImage":false,"uploadImage":null`, true},
		{`,"uploadImage":null,"uploadImage":false`, false},
	} {
		cfg, err := NewConfig(json.RawMessage(`{"userKey":"user","apiToken":"token"`+tc.fields+`}`), receiversTesting.DecryptForTesting(nil))
		require.NoError(t, err)
		require.Equal(t, tc.want, cfg.Upload)
		for _, codec := range []struct {
			marshal   func(any) ([]byte, error)
			unmarshal func([]byte, any) error
		}{
			{json.Marshal, json.Unmarshal}, {yaml.Marshal, yaml.Unmarshal},
		} {
			data, err := codec.marshal(cfg)
			require.NoError(t, err)
			var roundTrip Config
			require.NoError(t, codec.unmarshal(data, &roundTrip))
			require.Equal(t, cfg, roundTrip)
		}
	}
}

func TestConfigValidationOrder(t *testing.T) {
	for _, tc := range []struct {
		input string
		calls []string
		want  Config
		err   string
	}{
		{`{"priority":"bad"}`, []string{"userKey"}, Config{}, "user key not found"},
		{`{"userKey":"user","priority":"bad"}`, []string{"userKey", "apiToken"}, Config{UserKey: "user"}, "API token not found"},
		{`{"userKey":"user","apiToken":"token","priority":1,"okPriority":"bad","title":"ignored"}`, []string{"userKey", "apiToken"}, Config{UserKey: "user", APIToken: "token", AlertingPriority: 1}, "failed to convert OK priority"},
		{`{"uploadImage":"false"}`, nil, Config{}, "failed to unmarshal settings"},
	} {
		var calls []string
		cfg, err := NewConfig(json.RawMessage(tc.input), func(key, fallback string) (string, bool) {
			calls = append(calls, key)
			return fallback, false
		})
		require.ErrorContains(t, err, tc.err)
		require.Equal(t, tc.want, cfg)
		require.Equal(t, tc.calls, calls)
	}
}

func TestConfigYAMLCompatibility(t *testing.T) {
	for _, upload := range []string{"null", "true", "false"} {
		var cfg Config
		require.NoError(t, yaml.Unmarshal([]byte("priority: '1'\nokPriority: 2\nretry: invalid\nexpire: 1e2\nuploadImage: "+upload), &cfg))
		require.Equal(t, int64(1), cfg.AlertingPriority)
		require.Equal(t, int64(2), cfg.OkPriority)
		require.Zero(t, cfg.Retry)
		require.Zero(t, cfg.Expire)
		require.Equal(t, upload != "false", cfg.Upload)
	}
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte("defaults: &defaults {priority: '2', uploadImage: false}\n<<: *defaults"), &cfg))
	require.Equal(t, int64(2), cfg.AlertingPriority)
	require.False(t, cfg.Upload)
	require.Error(t, yaml.Unmarshal([]byte("priority: 1e2"), &cfg))
	require.Error(t, yaml.Unmarshal([]byte("uploadImage: false\nuploadImage: null"), &cfg))
}
