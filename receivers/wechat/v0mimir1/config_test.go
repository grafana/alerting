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

func TestWeChatTypeMatcher(t *testing.T) {
	good := []string{"text", "markdown"}
	for _, g := range good {
		if !wechatTypeMatcher.MatchString(g) {
			t.Fatalf("failed to match with %s", g)
		}
	}
	bad := []string{"TEXT", "MarkDOwn"}
	for _, b := range bad {
		if wechatTypeMatcher.MatchString(b) {
			t.Errorf("mistakenly match with %s", b)
		}
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
			expectedErr: "missing api_url in wechat config",
		},
		{
			name:        "Missing api_secret",
			mutate:      func(cfg *Config) { cfg.APISecret = "" },
			expectedErr: "missing api_secret in wechat config",
		},
		{
			name:        "Missing corp_id",
			mutate:      func(cfg *Config) { cfg.CorpID = "" },
			expectedErr: "missing corp_id in wechat config",
		},
		{
			name:        "Invalid message_type",
			mutate:      func(cfg *Config) { cfg.MessageType = "invalid" },
			expectedErr: "does not match valid options",
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
				"api_secret": "secret",
				"corp_id": "corp"
			}`,
			expectedInitError: "missing api_url in wechat config",
		},
		{
			name: "Error if missing api_secret",
			settings: `{
				"api_url": "http://localhost",
				"corp_id": "corp"
			}`,
			expectedInitError: "missing api_secret in wechat config",
		},
		{
			name: "Error if missing corp_id",
			settings: `{
				"api_url": "http://localhost",
				"api_secret": "secret"
			}`,
			expectedInitError: "missing corp_id in wechat config",
		},
		{
			name: "Minimal valid configuration",
			settings: `{
				"api_url": "http://localhost",
				"api_secret": "test-secret",
				"corp_id": "test-corp"
			}`,
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: false},
				HTTPConfig:     &defaultHTTPConfig,
				APISecret:      "test-secret",
				CorpID:         "test-corp",
				APIURL:         receivers.MustParseURL("http://localhost"),
				Message:        DefaultConfig.Message,
				ToUser:         DefaultConfig.ToUser,
				ToParty:        DefaultConfig.ToParty,
				ToTag:          DefaultConfig.ToTag,
				AgentID:        DefaultConfig.AgentID,
				MessageType:    "text",
			},
		},
		{
			name: "Secret from decrypt",
			settings: `{
				"api_url": "http://localhost",
				"corp_id": "test-corp"
			}`,
			secrets: map[string][]byte{
				"api_secret": []byte("secret-from-decrypt"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: false},
				HTTPConfig:     &defaultHTTPConfig,
				APISecret:      "secret-from-decrypt",
				CorpID:         "test-corp",
				APIURL:         receivers.MustParseURL("http://localhost"),
				Message:        DefaultConfig.Message,
				ToUser:         DefaultConfig.ToUser,
				ToParty:        DefaultConfig.ToParty,
				ToTag:          DefaultConfig.ToTag,
				AgentID:        DefaultConfig.AgentID,
				MessageType:    "text",
			},
		},
		{
			name: "Secret overrides setting",
			settings: `{
				"api_url": "http://localhost",
				"api_secret": "original",
				"corp_id": "test-corp"
			}`,
			secrets: map[string][]byte{
				"api_secret": []byte("override"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: false},
				HTTPConfig:     &defaultHTTPConfig,
				APISecret:      "override",
				CorpID:         "test-corp",
				APIURL:         receivers.MustParseURL("http://localhost"),
				Message:        DefaultConfig.Message,
				ToUser:         DefaultConfig.ToUser,
				ToParty:        DefaultConfig.ToParty,
				ToTag:          DefaultConfig.ToTag,
				AgentID:        DefaultConfig.AgentID,
				MessageType:    "text",
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
				"api_secret": []byte(string(GetFullValidConfig().APISecret)),
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
