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

func TestPushoverUserKeyIsPresent(t *testing.T) {
	in := `
user_key: ''
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "one of user_key or user_key_file must be configured"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}
}

func TestPushoverUserKeyOrUserKeyFile(t *testing.T) {
	in := `
user_key: 'user key'
user_key_file: /pushover/user_key
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "at most one of user_key & user_key_file must be configured"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}
}

func TestPushoverTokenIsPresent(t *testing.T) {
	in := `
user_key: '<user_key>'
token: ''
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "one of token or token_file must be configured"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}
}

func TestPushoverTokenOrTokenFile(t *testing.T) {
	in := `
token: 'pushover token'
token_file: /pushover/token
user_key: 'user key'
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "at most one of token & token_file must be configured"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
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
			name:        "Missing user_key",
			mutate:      func(cfg *Config) { cfg.UserKey = "" },
			expectedErr: "one of user_key or user_key_file must be configured",
		},
		{
			name:        "Missing token",
			mutate:      func(cfg *Config) { cfg.Token = "" },
			expectedErr: "one of token or token_file must be configured",
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
			name:              "Error if missing user_key",
			settings:          `{"token": "tok"}`,
			expectedInitError: "one of user_key or user_key_file must be configured",
		},
		{
			name:              "Error if missing token",
			settings:          `{"user_key": "key"}`,
			expectedInitError: "one of token or token_file must be configured",
		},
		{
			name: "Minimal valid configuration",
			settings: `{
				"user_key": "test-user-key",
				"token": "test-token"
			}`,
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				UserKey:        "test-user-key",
				Token:          "test-token",
				Title:          DefaultConfig.Title,
				Message:        DefaultConfig.Message,
				URL:            DefaultConfig.URL,
				Priority:       DefaultConfig.Priority,
				Retry:          DefaultConfig.Retry,
				Expire:         DefaultConfig.Expire,
			},
		},
		{
			name:     "Secrets from decrypt",
			settings: `{}`,
			secrets: map[string][]byte{
				"user_key": []byte("secret-user-key"),
				"token":    []byte("secret-token"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				UserKey:        "secret-user-key",
				Token:          "secret-token",
				Title:          DefaultConfig.Title,
				Message:        DefaultConfig.Message,
				URL:            DefaultConfig.URL,
				Priority:       DefaultConfig.Priority,
				Retry:          DefaultConfig.Retry,
				Expire:         DefaultConfig.Expire,
			},
		},
		{
			name: "Secrets override settings",
			settings: `{
				"user_key": "original-key",
				"token": "original-token"
			}`,
			secrets: map[string][]byte{
				"user_key": []byte("override-key"),
				"token":    []byte("override-token"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				UserKey:        "override-key",
				Token:          "override-token",
				Title:          DefaultConfig.Title,
				Message:        DefaultConfig.Message,
				URL:            DefaultConfig.URL,
				Priority:       DefaultConfig.Priority,
				Retry:          DefaultConfig.Retry,
				Expire:         DefaultConfig.Expire,
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
				"user_key": []byte(string(GetFullValidConfig().UserKey)),
				"token":    []byte(string(GetFullValidConfig().Token)),
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
