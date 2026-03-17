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
	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestTelegramConfiguration(t *testing.T) {
	tc := []struct {
		name     string
		in       string
		expected error
	}{
		{
			name: "with both bot_token & bot_token_file - it fails",
			in: `
bot_token: xyz
bot_token_file: /file
`,
			expected: errors.New("at most one of bot_token & bot_token_file must be configured"),
		},
		{
			name: "with no bot_token & bot_token_file - it fails",
			in: `
bot_token: ''
bot_token_file: ''
`,
			expected: errors.New("missing bot_token or bot_token_file on telegram_config"),
		},
		{
			name: "with bot_token and chat_id set - it succeeds",
			in: `
bot_token: xyz
chat_id: 123
`,
		},
		{
			name: "with bot_token_file and chat_id set - it succeeds",
			in: `
bot_token_file: /file
chat_id: 123
`,
		},
		{
			name: "with no chat_id set - it fails",
			in: `
bot_token: xyz
`,
			expected: errors.New("missing chat_id on telegram_config"),
		},
		{
			name: "with unknown parse_mode - it fails",
			in: `
bot_token: xyz
chat_id: 123
parse_mode: invalid
`,
			expected: errors.New("unknown parse_mode on telegram_config, must be Markdown, MarkdownV2, HTML or empty string"),
		},
		{
			name: "json tags are supported",
			in: `
token: xyz
chat: 123
parse_mode: Markdown
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
			name:        "Missing bot_token",
			mutate:      func(cfg *Config) { cfg.BotToken = "" },
			expectedErr: "missing bot_token or bot_token_file on telegram_config",
		},
		{
			name:        "Missing chat_id",
			mutate:      func(cfg *Config) { cfg.ChatID = 0 },
			expectedErr: "missing chat_id on telegram_config",
		},
		{
			name:        "Invalid parse_mode",
			mutate:      func(cfg *Config) { cfg.ParseMode = "invalid" },
			expectedErr: "unknown parse_mode on telegram_config",
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
			name:              "Error if missing bot_token",
			settings:          `{"chat": 123}`,
			expectedInitError: "missing bot_token or bot_token_file on telegram_config",
		},
		{
			name:              "Error if missing chat_id",
			settings:          `{"token": "tok"}`,
			expectedInitError: "missing chat_id on telegram_config",
		},
		{
			name: "Minimal valid configuration",
			settings: `{
				"token": "test-token",
				"chat": 123
			}`,
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				BotToken:       "test-token",
				ChatID:         123,
				Message:        DefaultConfig.Message,
				ParseMode:      DefaultConfig.ParseMode,
			},
		},
		{
			name:     "Secret token from decrypt",
			settings: `{"chat": 123}`,
			secrets: map[string][]byte{
				"token": []byte("secret-token"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				BotToken:       "secret-token",
				ChatID:         123,
				Message:        DefaultConfig.Message,
				ParseMode:      DefaultConfig.ParseMode,
			},
		},
		{
			name: "Secret overrides setting",
			settings: `{
				"token": "original",
				"chat": 123
			}`,
			secrets: map[string][]byte{
				"token": []byte("override"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				BotToken:       "override",
				ChatID:         123,
				Message:        DefaultConfig.Message,
				ParseMode:      DefaultConfig.ParseMode,
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
				"token": []byte(string(GetFullValidConfig().BotToken)),
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
