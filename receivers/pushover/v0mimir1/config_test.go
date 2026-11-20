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
	"time"

	"github.com/prometheus/alertmanager/config"
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

	expected := "missing user_key in Pushover config"

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

	expected := "missing token in Pushover config"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}
}

func TestPushoverFormattingForkParity(t *testing.T) {
	const credentials = "user_key: 'user key'\ntoken: 'pushover token'\n"
	for _, formatting := range []string{"", "html: true\n", "monospace: true\n", "html: true\nmonospace: true\n"} {
		t.Run(formatting, func(t *testing.T) {
			input := []byte(credentials + formatting)
			var fork config.PushoverConfig
			var local Config
			forkErr := yaml.UnmarshalStrict(input, &fork)
			localErr := yaml.UnmarshalStrict(input, &local)
			if formatting == "html: true\nmonospace: true\n" {
				const expected = "at most one of monospace & html must be configured"
				require.EqualError(t, forkErr, expected)
				require.EqualError(t, localErr, expected)
				return
			}
			require.NoError(t, forkErr)
			require.NoError(t, localErr)
			require.Equal(t, fork.HTML, local.HTML)
			require.Equal(t, fork.Monospace, local.Monospace)

			// Compare the formatting keys only. The local config has its own wire types.
			for _, cfg := range []any{fork, local} {
				encoded, err := yaml.Marshal(cfg)
				require.NoError(t, err)
				var fields map[string]any
				require.NoError(t, yaml.Unmarshal(encoded, &fields))
				require.Equal(t, fork.HTML, fields["html"] == true)
				require.Equal(t, fork.Monospace, fields["monospace"] == true)
				require.Equal(t, fork.HTML, fields["html"] != nil)
				require.Equal(t, fork.Monospace, fields["monospace"] != nil)
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
			name:        "Missing user_key",
			mutate:      func(cfg *Config) { cfg.UserKey = "" },
			expectedErr: "missing user_key in Pushover config",
		},
		{
			name:        "Missing token",
			mutate:      func(cfg *Config) { cfg.Token = "" },
			expectedErr: "missing token in Pushover config",
		},
		{
			name:        "HTML and monospace conflict",
			mutate:      func(cfg *Config) { cfg.HTML = true },
			expectedErr: "at most one of monospace & html must be configured",
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
			expectedInitError: "missing user_key in Pushover config",
		},
		{
			name:              "Error if missing token",
			settings:          `{"user_key": "key"}`,
			expectedInitError: "missing token in Pushover config",
		},
		{
			name:              "HTML and monospace conflict",
			settings:          `{"user_key":"key","token":"tok","html":true,"monospace":true}`,
			expectedInitError: "at most one of monospace & html must be configured",
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
			name:     "Fractional durations and decrypted credentials",
			settings: `{"retry":"30.5s","expire":"1m0.25s","ttl":"0.75s"}`,
			secrets: map[string][]byte{
				"user_key": []byte("test-user-key"),
				"token":    []byte("test-token"),
			},
			expectedConfig: func() Config {
				cfg := DefaultConfig
				cfg.HTTPConfig = &defaultHTTPConfig
				cfg.UserKey = "test-user-key"
				cfg.Token = "test-token"
				cfg.Retry = FractionalDuration(30*time.Second + 500*time.Millisecond)
				cfg.Expire = FractionalDuration(time.Minute + 250*time.Millisecond)
				cfg.TTL = FractionalDuration(750 * time.Millisecond)
				return cfg
			}(),
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
