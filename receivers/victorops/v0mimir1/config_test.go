// Copyright 2018 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v0mimir1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	httpcfg "github.com/grafana/alerting/http/v0mimir"
	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestVictorOpsConfiguration(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		in := `
routing_key: test
api_key_file: /global_file
`
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(in), &cfg)
		if err != nil {
			t.Fatalf("no error was expected:\n%v", err)
		}
	})

	t.Run("routing key is missing", func(t *testing.T) {
		in := `
routing_key: ''
`
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(in), &cfg)

		expected := "missing Routing key in VictorOps config"

		if err == nil {
			t.Fatalf("no error returned, expected:\n%v", expected)
		}
		if err.Error() != expected {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
		}
	})

	t.Run("api_key and api_key_file both defined", func(t *testing.T) {
		in := `
routing_key: test
api_key: xyz
api_key_file: /global_file
`
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(in), &cfg)

		expected := "at most one of api_key & api_key_file must be configured"

		if err == nil {
			t.Fatalf("no error returned, expected:\n%v", expected)
		}
		if err.Error() != expected {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
		}
	})
}

func TestVictorOpsCustomFieldsValidation(t *testing.T) {
	in := `
routing_key: 'test'
custom_fields:
  entity_state: 'state_message'
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "victorOps config contains custom field entity_state which cannot be used as it conflicts with the fixed/static fields"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}

	in = `
routing_key: 'test'
custom_fields:
  my_special_field: 'special_label'
`

	err = yaml.UnmarshalStrict([]byte(in), &cfg)

	expected = "special_label"

	if err != nil {
		t.Fatalf("Unexpected error returned, got:\n%v", err.Error())
	}

	val, ok := cfg.CustomFields["my_special_field"]

	if !ok {
		t.Fatalf("Expected Custom Field to have value %v set, field is empty", expected)
	}
	if val != expected {
		t.Errorf("\nexpected custom field my_special_field value:\n%v\ngot:\n%v", expected, val)
	}
}

func TestValidate(t *testing.T) {
	t.Run("FullValidConfigForTesting is valid", func(t *testing.T) {
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(FullValidConfigForTesting), &cfg)
		require.NoError(t, err)
		require.NoError(t, cfg.Validate())
	})
	cases := []struct {
		name        string
		mutate      func(cfg *Config)
		expectedErr string
	}{
		{
			name:   "GetFullValidConfig is valid",
			mutate: func(cfg *Config) {},
		},
		{
			name:        "Missing routing_key",
			mutate:      func(cfg *Config) { cfg.RoutingKey = "" },
			expectedErr: "missing Routing key in VictorOps config",
		},
		{
			name:        "Missing api_url",
			mutate:      func(cfg *Config) { cfg.APIURL = nil },
			expectedErr: "missing api_url in VictorOps config",
		},
		{
			name:        "Missing api_key",
			mutate:      func(cfg *Config) { cfg.APIKey = "" },
			expectedErr: "missing api_key in VictorOps config",
		},
		{
			name: "Reserved custom field",
			mutate: func(cfg *Config) {
				cfg.CustomFields = map[string]string{"entity_state": "value"}
			},
			expectedErr: "conflicts with the fixed/static fields",
		},
		{
			name: "Invalid http_config",
			mutate: func(cfg *Config) {
				cfg.HTTPConfig = &httpcfg.HTTPClientConfig{
					BasicAuth: &httpcfg.BasicAuth{},
					OAuth2:    &httpcfg.OAuth2{},
				}
			},
			expectedErr: "invalid http_config",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := GetFullValidConfig()
			c.mutate(&cfg)
			err := cfg.Validate()

			if c.expectedErr != "" {
				require.ErrorContains(t, err, c.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNewConfig(t *testing.T) {
	defaultHTTPConfig := httpcfg.DefaultHTTPClientConfig

	cases := []struct {
		name              string
		settings          string
		secrets           map[string][]byte
		expectedConfig    Config
		expectedInitError string
	}{
		{
			name:              "Error if empty",
			settings:          "",
			expectedInitError: "failed to unmarshal settings",
		},
		{
			name: "Error if missing routing_key",
			settings: `{
				"api_url": "http://localhost",
				"api_key": "key"
			}`,
			expectedInitError: "missing Routing key in VictorOps config",
		},
		{
			name: "Error if missing api_url",
			settings: `{
				"api_key": "key",
				"routing_key": "team1"
			}`,
			expectedInitError: "missing api_url in VictorOps config",
		},
		{
			name: "Error if missing api_key",
			settings: `{
				"api_url": "http://localhost",
				"routing_key": "team1"
			}`,
			expectedInitError: "missing api_key in VictorOps config",
		},
		{
			name: "Minimal valid configuration",
			settings: `{
				"api_url": "http://localhost",
				"api_key": "test-key",
				"routing_key": "team1"
			}`,
			expectedConfig: Config{
				NotifierConfig:    receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:        &defaultHTTPConfig,
				APIKey:            "test-key",
				APIURL:            receivers.MustParseURL("http://localhost"),
				RoutingKey:        "team1",
				MessageType:       DefaultConfig.MessageType,
				StateMessage:      DefaultConfig.StateMessage,
				EntityDisplayName: DefaultConfig.EntityDisplayName,
				MonitoringTool:    DefaultConfig.MonitoringTool,
			},
		},
		{
			name: "Secret from decrypt",
			settings: `{
				"api_url": "http://localhost",
				"routing_key": "team1"
			}`,
			secrets: map[string][]byte{
				"api_key": []byte("secret-key"),
			},
			expectedConfig: Config{
				NotifierConfig:    receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:        &defaultHTTPConfig,
				APIKey:            "secret-key",
				APIURL:            receivers.MustParseURL("http://localhost"),
				RoutingKey:        "team1",
				MessageType:       DefaultConfig.MessageType,
				StateMessage:      DefaultConfig.StateMessage,
				EntityDisplayName: DefaultConfig.EntityDisplayName,
				MonitoringTool:    DefaultConfig.MonitoringTool,
			},
		},
		{
			name: "Secret overrides setting",
			settings: `{
				"api_url": "http://localhost",
				"api_key": "original",
				"routing_key": "team1"
			}`,
			secrets: map[string][]byte{
				"api_key": []byte("override"),
			},
			expectedConfig: Config{
				NotifierConfig:    receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:        &defaultHTTPConfig,
				APIKey:            "override",
				APIURL:            receivers.MustParseURL("http://localhost"),
				RoutingKey:        "team1",
				MessageType:       DefaultConfig.MessageType,
				StateMessage:      DefaultConfig.StateMessage,
				EntityDisplayName: DefaultConfig.EntityDisplayName,
				MonitoringTool:    DefaultConfig.MonitoringTool,
			},
		},
		{
			name:     "FullValidConfigForTesting is valid",
			settings: FullValidConfigForTesting,
			expectedConfig: func() Config {
				cfg := DefaultConfig
				_ = json.Unmarshal([]byte(FullValidConfigForTesting), &cfg)
				httpCfg := httpcfg.DefaultHTTPClientConfig
				cfg.HTTPConfig = &httpCfg
				return cfg
			}(),
		},
		{
			name:     "GetFullValidConfig round-trips through JSON",
			settings: func() string { b, _ := json.Marshal(GetFullValidConfig()); return string(b) }(),
			secrets: map[string][]byte{
				"api_key": []byte(string(GetFullValidConfig().APIKey)),
			},
			expectedConfig: func() Config {
				cfg := GetFullValidConfig()
				cfg.HTTPConfig = &defaultHTTPConfig
				return cfg
			}(),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(c.secrets))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
