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

func TestSNS(t *testing.T) {
	for _, tc := range []struct {
		in  string
		err bool
	}{
		{
			// Valid configuration without sigv4.
			in:  `target_arn: target`,
			err: false,
		},
		{
			// Valid configuration without sigv4.
			in:  `topic_arn: topic`,
			err: false,
		},
		{
			// Valid configuration with sigv4.
			in: `phone_number: phone
sigv4:
    access_key: abc
    secret_key: abc
`,
			err: false,
		},
		{
			// at most one of 'target_arn', 'topic_arn' or 'phone_number' must be provided without sigv4.
			in: `topic_arn: topic
target_arn: target
`,
			err: true,
		},
		{
			// at most one of 'target_arn', 'topic_arn' or 'phone_number' must be provided without sigv4.
			in: `topic_arn: topic
phone_number: phone
`,
			err: true,
		},
		{
			// one of 'target_arn', 'topic_arn' or 'phone_number' must be provided without sigv4.
			in:  "{}",
			err: true,
		},
		{
			// one of 'target_arn', 'topic_arn' or 'phone_number' must be provided with sigv4.
			in: `sigv4:
    access_key: abc
    secret_key: abc
`,
			err: true,
		},
		{
			// 'secret_key' must be provided with 'access_key'.
			in: `topic_arn: topic
sigv4:
    access_key: abc
`,
			err: true,
		},
		{
			// 'access_key' must be provided with 'secret_key'.
			in: `topic_arn: topic
sigv4:
    secret_key: abc
`,
			err: true,
		},
	} {
		t.Run("", func(t *testing.T) {
			var cfg Config
			err := yaml.UnmarshalStrict([]byte(tc.in), &cfg)
			if err != nil {
				if !tc.err {
					t.Errorf("expecting no error, got %q", err)
				}
				return
			}

			if tc.err {
				t.Logf("%#v", cfg)
				t.Error("expecting error, got none")
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
			name: "Missing target - must provide one of target_arn, topic_arn, or phone_number",
			mutate: func(cfg *Config) {
				cfg.TopicARN = ""
				cfg.TargetARN = ""
				cfg.PhoneNumber = ""
			},
			expectedErr: "must provide either a Target ARN, Topic ARN, or Phone Number",
		},
		{
			name: "Both topic_arn and target_arn",
			mutate: func(cfg *Config) {
				cfg.PhoneNumber = ""
			},
			expectedErr: "must provide either a Target ARN, Topic ARN, or Phone Number",
		},
		{
			name: "SigV4 access_key without secret_key",
			mutate: func(cfg *Config) {
				cfg.Sigv4.SecretKey = ""
			},
			expectedErr: "must provide a AWS SigV4 Access key and Secret Key",
		},
		{
			name: "SigV4 secret_key without access_key",
			mutate: func(cfg *Config) {
				cfg.Sigv4.AccessKey = ""
			},
			expectedErr: "must provide a AWS SigV4 Access key and Secret Key",
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
			name:              "Error if missing target",
			settings:          `{}`,
			expectedInitError: "must provide either a Target ARN, Topic ARN, or Phone Number",
		},
		{
			name: "Minimal valid with topic_arn",
			settings: `{
				"topic_arn": "arn:aws:sns:us-east-1:123456789:test"
			}`,
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				TopicARN:       "arn:aws:sns:us-east-1:123456789:test",
				Subject:        DefaultConfig.Subject,
				Message:        DefaultConfig.Message,
			},
		},
		{
			name: "Secret sigv4.secret_key from decrypt",
			settings: `{
				"topic_arn": "arn:aws:sns:us-east-1:123456789:test",
				"sigv4": {"access_key": "ak"}
			}`,
			secrets: map[string][]byte{
				"sigv4.secret_key": []byte("secret-sk"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				TopicARN:       "arn:aws:sns:us-east-1:123456789:test",
				Sigv4:          SigV4Config{AccessKey: "ak", SecretKey: "secret-sk"},
				Subject:        DefaultConfig.Subject,
				Message:        DefaultConfig.Message,
			},
		},
		{
			name: "Secret overrides setting",
			settings: `{
				"topic_arn": "arn:aws:sns:us-east-1:123456789:test",
				"sigv4": {"access_key": "ak", "secret_key": "original"}
			}`,
			secrets: map[string][]byte{
				"sigv4.secret_key": []byte("override"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				TopicARN:       "arn:aws:sns:us-east-1:123456789:test",
				Sigv4:          SigV4Config{AccessKey: "ak", SecretKey: "override"},
				Subject:        DefaultConfig.Subject,
				Message:        DefaultConfig.Message,
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
				"sigv4.secret_key": []byte(string(GetFullValidConfig().Sigv4.SecretKey)),
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
