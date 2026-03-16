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

func TestPagerdutyTestRoutingKey(t *testing.T) {
	t.Run("error if no routing key or key file", func(t *testing.T) {
		in := `
routing_key: ''
`
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(in), &cfg)

		expected := "missing service or routing key in PagerDuty config"

		if err == nil {
			t.Fatalf("no error returned, expected:\n%v", expected)
		}
		if err.Error() != expected {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
		}
	})

	t.Run("error if both routing key and key file", func(t *testing.T) {
		in := `
routing_key: 'xyz'
routing_key_file: 'xyz'
`
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(in), &cfg)

		expected := "at most one of routing_key & routing_key_file must be configured"

		if err == nil {
			t.Fatalf("no error returned, expected:\n%v", expected)
		}
		if err.Error() != expected {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
		}
	})
}

func TestPagerdutyServiceKey(t *testing.T) {
	t.Run("error if no service key or key file", func(t *testing.T) {
		in := `
service_key: ''
`
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(in), &cfg)

		expected := "missing service or routing key in PagerDuty config"

		if err == nil {
			t.Fatalf("no error returned, expected:\n%v", expected)
		}
		if err.Error() != expected {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
		}
	})

	t.Run("error if both service key and key file", func(t *testing.T) {
		in := `
service_key: 'xyz'
service_key_file: 'xyz'
`
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(in), &cfg)

		expected := "at most one of service_key & service_key_file must be configured"

		if err == nil {
			t.Fatalf("no error returned, expected:\n%v", expected)
		}
		if err.Error() != expected {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
		}
	})
}

func TestPagerdutyDetails(t *testing.T) {
	tests := []struct {
		in      string
		checkFn func(map[string]string)
	}{
		{
			in: `
routing_key: 'xyz'
`,
			checkFn: func(d map[string]string) {
				if len(d) != 4 {
					t.Errorf("expected 4 items, got: %d", len(d))
				}
			},
		},
		{
			in: `
routing_key: 'xyz'
details:
  key1: val1
`,
			checkFn: func(d map[string]string) {
				if len(d) != 5 {
					t.Errorf("expected 5 items, got: %d", len(d))
				}
			},
		},
		{
			in: `
routing_key: 'xyz'
details:
  key1: val1
  key2: val2
  firing: firing
`,
			checkFn: func(d map[string]string) {
				if len(d) != 6 {
					t.Errorf("expected 6 items, got: %d", len(d))
				}
			},
		},
	}
	for _, tc := range tests {
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(tc.in), &cfg)
		if err != nil {
			t.Errorf("expected no error, got:%v", err)
		}

		if tc.checkFn != nil {
			tc.checkFn(cfg.Details)
		}
	}
}

func TestPagerDutySource(t *testing.T) {
	for _, tc := range []struct {
		title string
		in    string

		expectedSource string
	}{
		{
			title: "check source field is backward compatible",
			in: `
routing_key: 'xyz'
client: 'alert-manager-client'
`,
			expectedSource: "alert-manager-client",
		},
		{
			title: "check source field is set",
			in: `
routing_key: 'xyz'
client: 'alert-manager-client'
source: 'alert-manager-source'
`,
			expectedSource: "alert-manager-source",
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			var cfg Config
			err := yaml.UnmarshalStrict([]byte(tc.in), &cfg)
			require.NoError(t, err)
			require.Equal(t, tc.expectedSource, cfg.Source)
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
			name:        "Missing url",
			mutate:      func(cfg *Config) { cfg.URL = nil },
			expectedErr: "missing url in PagerDuty config",
		},
		{
			name: "Missing service or routing key",
			mutate: func(cfg *Config) {
				cfg.RoutingKey = ""
				cfg.ServiceKey = ""
			},
			expectedErr: "missing service or routing key in PagerDuty config",
		},
		{
			name: "Both routing_key and routing_key_file",
			mutate: func(cfg *Config) {
				cfg.RoutingKeyFile = "file"
			},
			expectedErr: "at most one of routing_key & routing_key_file must be configured",
		},
		{
			name: "Both service_key and service_key_file",
			mutate: func(cfg *Config) {
				cfg.ServiceKeyFile = "file"
			},
			expectedErr: "at most one of service_key & service_key_file must be configured",
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
			name: "Error if missing url",
			settings: `{
				"routing_key": "key"
			}`,
			expectedInitError: "missing url in PagerDuty config",
		},
		{
			name: "Error if missing routing/service key",
			settings: `{
				"url": "http://localhost"
			}`,
			expectedInitError: "missing service or routing key in PagerDuty config",
		},
		{
			name: "Minimal valid with routing_key",
			settings: `{
				"url": "http://localhost",
				"routing_key": "test-key"
			}`,
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				URL:            receivers.MustParseURL("http://localhost"),
				RoutingKey:     "test-key",
				Description:    DefaultConfig.Description,
				Client:         DefaultConfig.Client,
				ClientURL:      DefaultConfig.ClientURL,
			},
		},
		{
			name: "Minimal valid with service_key",
			settings: `{
				"url": "http://localhost",
				"service_key": "test-key"
			}`,
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				URL:            receivers.MustParseURL("http://localhost"),
				ServiceKey:     "test-key",
				Description:    DefaultConfig.Description,
				Client:         DefaultConfig.Client,
				ClientURL:      DefaultConfig.ClientURL,
			},
		},
		{
			name: "Secrets from decrypt",
			settings: `{
				"url": "http://localhost"
			}`,
			secrets: map[string][]byte{
				"routing_key": []byte("secret-routing"),
				"service_key": []byte("secret-service"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				URL:            receivers.MustParseURL("http://localhost"),
				RoutingKey:     "secret-routing",
				ServiceKey:     "secret-service",
				Description:    DefaultConfig.Description,
				Client:         DefaultConfig.Client,
				ClientURL:      DefaultConfig.ClientURL,
			},
		},
		{
			name: "Secrets override settings",
			settings: `{
				"url": "http://localhost",
				"routing_key": "original-routing",
				"service_key": "original-service"
			}`,
			secrets: map[string][]byte{
				"routing_key": []byte("override-routing"),
				"service_key": []byte("override-service"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				URL:            receivers.MustParseURL("http://localhost"),
				RoutingKey:     "override-routing",
				ServiceKey:     "override-service",
				Description:    DefaultConfig.Description,
				Client:         DefaultConfig.Client,
				ClientURL:      DefaultConfig.ClientURL,
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
				"routing_key": []byte(string(GetFullValidConfig().RoutingKey)),
				"service_key": []byte(string(GetFullValidConfig().ServiceKey)),
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
