package v1

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestConfigBooleanDefaults(t *testing.T) {
	values := []struct {
		wire string
		want bool
	}{
		{"", true}, {"null", true}, {"true", true}, {"false", false},
	}
	for _, autoClose := range values {
		for _, overridePriority := range values {
			t.Run(autoClose.wire+"/"+overridePriority.wire, func(t *testing.T) {
				jsonInput := `{"apiKey":"key"`
				yamlInput := "apiKey: key\n"
				if autoClose.wire != "" {
					jsonInput += `,"autoClose":` + autoClose.wire
					yamlInput += "autoClose: " + autoClose.wire + "\n"
				}
				if overridePriority.wire != "" {
					jsonInput += `,"overridePriority":` + overridePriority.wire
					yamlInput += "overridePriority: " + overridePriority.wire + "\n"
				}
				jsonInput += "}"
				cfg, err := NewConfig(json.RawMessage(jsonInput), receiversTesting.DecryptForTesting(nil))
				require.NoError(t, err)
				require.Equal(t, autoClose.want, cfg.AutoClose)
				require.Equal(t, overridePriority.want, cfg.OverridePriority)

				var yamlConfig Config
				require.NoError(t, yaml.Unmarshal([]byte(yamlInput), &yamlConfig))
				require.Equal(t, autoClose.want, yamlConfig.AutoClose)
				require.Equal(t, overridePriority.want, yamlConfig.OverridePriority)

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
			})
		}
	}
}

func TestConfigDuplicateBooleanKeys(t *testing.T) {
	for _, key := range []string{"autoClose", "overridePriority"} {
		for _, tc := range []struct {
			first string
			last  string
			want  bool
		}{
			{"false", "null", true}, {"true", "false", false},
			{"null", "false", false}, {"false", "true", true},
		} {
			cfg, err := NewConfig(json.RawMessage(fmt.Sprintf(`{"apiKey":"key",%q:%s,%q:%s}`, key, tc.first, key, tc.last)), receiversTesting.DecryptForTesting(nil))
			require.NoError(t, err)
			if key == "autoClose" {
				require.Equal(t, tc.want, cfg.AutoClose)
				require.True(t, cfg.OverridePriority)
			} else {
				require.Equal(t, tc.want, cfg.OverridePriority)
				require.True(t, cfg.AutoClose)
			}
		}
	}
}

func TestConfigInvalidBooleans(t *testing.T) {
	for _, key := range []string{"autoClose", "overridePriority"} {
		for _, value := range []string{`"false"`, `0`, `[]`, `{}`} {
			cfg, err := NewConfig(json.RawMessage(fmt.Sprintf(`{"apiKey":"key",%q:%s}`, key, value)), receiversTesting.DecryptForTesting(nil))
			require.ErrorContains(t, err, "failed to unmarshal settings")
			require.Zero(t, cfg)
		}
	}
}

func TestConfigPreservesResponderCase(t *testing.T) {
	cfg, err := NewConfig(json.RawMessage(`{"apiKey":"key","responders":[{"type":"TEAM","name":"team"}]}`), receiversTesting.DecryptForTesting(nil))
	require.NoError(t, err)
	require.Equal(t, "TEAM", cfg.Responders[0].Type)
}

func TestConfigYAMLDefaultsWithMerge(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte("defaults: &defaults {autoClose: false}\n<<: *defaults\napiKey: key\noverridePriority: null\n"), &cfg))
	require.False(t, cfg.AutoClose)
	require.True(t, cfg.OverridePriority)
	require.Error(t, yaml.Unmarshal([]byte("autoClose: false\nautoClose: null"), &cfg))
}
