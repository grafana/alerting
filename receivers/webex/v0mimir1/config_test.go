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
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	httpcfg "github.com/grafana/alerting/http/v0mimir"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestWebexConfiguration(t *testing.T) {
	tc := []struct {
		name string

		in       string
		expected error
	}{
		{
			name: "with no room_id - it fails",
			in: `
message: xyz123
`,
			expected: errors.New("missing room_id on webex_config"),
		},
		{
			name: "with room_id and http_config.authorization set - it succeeds",
			in: `
room_id: 2
http_config:
  authorization:
    credentials: "xxxyyyzz"
`,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			err := yaml.UnmarshalStrict([]byte(tt.in), &cfg)

			require.Equal(t, tt.expected, err)
		})
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
			name:        "Missing room_id",
			mutate:      func(cfg *Config) { cfg.RoomID = "" },
			expectedErr: "missing room_id on webex_config",
		},
		{
			name:        "Missing api_url",
			mutate:      func(cfg *Config) { cfg.APIURL = nil },
			expectedErr: "missing api_url in webex config",
		},
		{
			name: "Missing http_config.authorization",
			mutate: func(cfg *Config) {
				cfg.HTTPConfig.Authorization = nil
			},
			expectedErr: "missing webex_configs.http_config.authorization",
		},
		{
			name: "Invalid http_config",
			mutate: func(cfg *Config) {
				cfg.HTTPConfig.BasicAuth = &httpcfg.BasicAuth{}
				cfg.HTTPConfig.OAuth2 = &httpcfg.OAuth2{}
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
			name:              "Error if missing room_id",
			settings:          `{"api_url": "https://localhost", "http_config": {"authorization": {"credentials": "tok"}}}`,
			expectedInitError: "missing room_id on webex_config",
		},
		{
			name:              "Error if missing authorization",
			settings:          `{"api_url": "https://localhost", "room_id": "123"}`,
			expectedInitError: "missing webex_configs.http_config.authorization",
		},
		{
			name:     "Valid configuration",
			settings: FullValidConfigForTesting,
			expectedConfig: func() Config {
				cfg := DefaultConfig
				_ = json.Unmarshal([]byte(FullValidConfigForTesting), &cfg)
				// DecryptHTTPConfig always returns non-nil
				httpCfg, _ := httpcfg.DecryptHTTPConfig("http_config", cfg.HTTPConfig, receiversTesting.DecryptForTesting(nil))
				cfg.HTTPConfig = httpCfg
				return cfg
			}(),
		},
		{
			name: "Authorization credentials from decrypt",
			settings: `{
				"api_url": "https://localhost",
				"room_id": "123"
			}`,
			secrets: map[string][]byte{
				"http_config.authorization.credentials": []byte("secret-token"),
			},
			expectedConfig: func() Config {
				cfg := DefaultConfig
				_ = json.Unmarshal([]byte(`{"api_url": "https://localhost", "room_id": "123"}`), &cfg)
				httpCfg, _ := httpcfg.DecryptHTTPConfig("http_config", cfg.HTTPConfig, receiversTesting.DecryptForTesting(map[string][]byte{
					"http_config.authorization.credentials": []byte("secret-token"),
				}))
				// Validate() sets default Authorization.Type to "Bearer"
				_ = httpCfg.Validate()
				cfg.HTTPConfig = httpCfg
				return cfg
			}(),
		},
		{
			name:     "GetFullValidConfig round-trips through JSON",
			settings: func() string { b, _ := json.Marshal(GetFullValidConfig()); return string(b) }(),
			secrets: map[string][]byte{
				"http_config.authorization.credentials": []byte(string(GetFullValidConfig().HTTPConfig.Authorization.Credentials)),
			},
			expectedConfig: func() Config {
				cfg := GetFullValidConfig()
				httpCfg, _ := httpcfg.DecryptHTTPConfig("http_config", cfg.HTTPConfig, receiversTesting.DecryptForTesting(map[string][]byte{
					"http_config.authorization.credentials": []byte(string(cfg.HTTPConfig.Authorization.Credentials)),
				}))
				cfg.HTTPConfig = httpCfg
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
