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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	httpcfg "github.com/grafana/alerting/http/v0mimir"
	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestWebhookURLIsPresent(t *testing.T) {
	in := `{}`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "one of url or url_file must be configured"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}
}

func TestWebhookURLOrURLFile(t *testing.T) {
	in := `
url: 'http://example.com'
url_file: 'http://example.com'
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "at most one of url & url_file must be configured"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}
}

func TestWebhookHttpConfigIsValid(t *testing.T) {
	in := `
url: 'http://example.com'
http_config:
  bearer_token: foo
  bearer_token_file: /tmp/bar
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "at most one of bearer_token & bearer_token_file must be configured"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}
}

func TestWebhookHttpConfigIsOptional(t *testing.T) {
	in := `
url: 'http://example.com'
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)
	if err != nil {
		t.Fatalf("no error expected, returned:\n%v", err.Error())
	}
}

func TestWebhookPasswordIsObfuscated(t *testing.T) {
	in := `
url: 'http://example.com'
http_config:
  basic_auth:
    username: foo
    password: supersecret
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)
	if err != nil {
		t.Fatalf("no error expected, returned:\n%v", err.Error())
	}

	ycfg, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("no error expected, returned:\n%v", err.Error())
	}
	if strings.Contains(string(ycfg), "supersecret") {
		t.Errorf("Found password in the YAML cfg: %s\n", ycfg)
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
			expectedErr: "one of url or url_file must be configured",
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
			name:              "Error if missing url",
			settings:          `{}`,
			expectedInitError: "one of url or url_file must be configured",
		},
		{
			name: "Minimal valid configuration",
			settings: `{
				"url": "http://webhook.example.com"
			}`,
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				URL:            receivers.MustParseSecretURL("http://webhook.example.com"),
			},
		},
		{
			name:     "Secret url from decrypt",
			settings: `{}`,
			secrets: map[string][]byte{
				"url": []byte("http://webhook.example.com/secret"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				URL:            receivers.MustParseSecretURL("http://webhook.example.com/secret"),
			},
		},
		{
			name: "Secret overrides setting",
			settings: `{
				"url": "http://original.example.com"
			}`,
			secrets: map[string][]byte{
				"url": []byte("http://override.example.com"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				URL:            receivers.MustParseSecretURL("http://override.example.com"),
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
				"url": []byte(GetFullValidConfig().URL.String()),
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
