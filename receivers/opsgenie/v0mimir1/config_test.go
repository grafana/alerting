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

func TestOpsgenieTypeMatcher(t *testing.T) {
	good := []string{"team", "user", "escalation", "schedule"}
	for _, g := range good {
		if !opsgenieTypeMatcher.MatchString(g) {
			t.Fatalf("failed to match with %s", g)
		}
	}
	bad := []string{"0user", "team1", "2escalation3", "sche4dule", "User", "TEAM"}
	for _, b := range bad {
		if opsgenieTypeMatcher.MatchString(b) {
			t.Errorf("mistakenly match with %s", b)
		}
	}
}

func TestOpsGenieConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string

		err bool
	}{
		{
			name: "valid configuration",
			in: `api_key: xyz
responders:
- id: foo
  type: scheDule
- name: bar
  type: teams
- username: fred
  type: USER
api_url: http://example.com
`,
		},
		{
			name: "api_key and api_key_file both defined",
			in: `api_key: xyz
api_key_file: xyz
api_url: http://example.com
`,
			err: true,
		},
		{
			name: "invalid responder type",
			in: `api_key: xyz
responders:
- id: foo
  type: wrong
api_url: http://example.com
`,
			err: true,
		},
		{
			name: "missing responder field",
			in: `api_key: xyz
responders:
- type: schedule
api_url: http://example.com
`,
			err: true,
		},
		{
			name: "valid responder type template",
			in: `api_key: xyz
responders:
- id: foo
  type: "{{/* valid comment */}}team"
api_url: http://example.com
`,
		},
		{
			name: "invalid responder type template",
			in: `api_key: xyz
responders:
- id: foo
  type: "{{/* invalid comment }}team"
api_url: http://example.com
`,
			err: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config

			err := yaml.UnmarshalStrict([]byte(tc.in), &cfg)
			if tc.err {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
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
			name:        "Missing api_url",
			mutate:      func(cfg *Config) { cfg.APIURL = nil },
			expectedErr: "missing api_url in opsgenie config",
		},
		{
			name:        "Missing api_key",
			mutate:      func(cfg *Config) { cfg.APIKey = "" },
			expectedErr: "missing api_key in opsgenie config",
		},
		{
			name: "Invalid responder - missing id/name/username",
			mutate: func(cfg *Config) {
				cfg.Responders = []Responder{{Type: "team"}}
			},
			expectedErr: "has to have at least one of id, username or name specified",
		},
		{
			name: "Invalid responder type",
			mutate: func(cfg *Config) {
				cfg.Responders = []Responder{{ID: "foo", Type: "wrong"}}
			},
			expectedErr: "type does not match valid options",
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
			name: "Error if missing api_url",
			settings: `{
				"api_key": "key",
				"project": "PROJ"
			}`,
			expectedInitError: "missing api_url in opsgenie config",
		},
		{
			name: "Error if missing api_key",
			settings: `{
				"api_url": "http://localhost"
			}`,
			expectedInitError: "missing api_key in opsgenie config",
		},
		{
			name: "Minimal valid configuration",
			settings: `{
				"api_key": "test-key",
				"api_url": "http://localhost"
			}`,
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				APIKey:         "test-key",
				APIURL:         receivers.MustParseURL("http://localhost"),
				Message:        DefaultConfig.Message,
				Description:    DefaultConfig.Description,
				Source:         DefaultConfig.Source,
			},
		},
		{
			name: "Secret from decrypt",
			settings: `{
				"api_url": "http://localhost"
			}`,
			secrets: map[string][]byte{
				"api_key": []byte("secret-key"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				APIKey:         "secret-key",
				APIURL:         receivers.MustParseURL("http://localhost"),
				Message:        DefaultConfig.Message,
				Description:    DefaultConfig.Description,
				Source:         DefaultConfig.Source,
			},
		},
		{
			name: "Secret overrides setting",
			settings: `{
				"api_key": "original",
				"api_url": "http://localhost"
			}`,
			secrets: map[string][]byte{
				"api_key": []byte("override"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				APIKey:         "override",
				APIURL:         receivers.MustParseURL("http://localhost"),
				Message:        DefaultConfig.Message,
				Description:    DefaultConfig.Description,
				Source:         DefaultConfig.Source,
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
