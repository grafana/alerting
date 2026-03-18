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
	"net/mail"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	commoncfg "github.com/prometheus/common/config"

	httpcfg "github.com/grafana/alerting/http/v0mimir"
	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestEmailToIsPresent(t *testing.T) {
	in := `
to: ''
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "missing to address in email config"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}
}

func TestEmailHeadersCollision(t *testing.T) {
	in := `
to: 'to@email.com'
headers:
  Subject: 'Alert'
  subject: 'New Alert'
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "duplicate header \"Subject\" in email config"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}
}

func TestEmailToAllowsMultipleAdresses(t *testing.T) {
	in := `
to: 'a@example.com, ,b@example.com,c@example.com'
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)
	if err != nil {
		t.Fatal(err)
	}

	expected := []*mail.Address{
		{Address: "a@example.com"},
		{Address: "b@example.com"},
		{Address: "c@example.com"},
	}

	res, err := mail.ParseAddressList(cfg.To)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(res, expected) {
		t.Fatalf("expected %v, got %v", expected, res)
	}
}

func TestEmailDisallowMalformed(t *testing.T) {
	in := `
to: 'a@'
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = mail.ParseAddressList(cfg.To)
	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", "mail: no angle-addr")
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
			name: "Error if missing to",
			settings: `{
				"from": "a@b.com",
				"smarthost": "smtp:587"
			}`,
			expectedInitError: "missing to address in email config",
		},
		{
			name: "Error if missing smarthost",
			settings: `{
				"to": "a@b.com",
				"from": "b@b.com"
			}`,
			expectedInitError: "missing smarthost in email config",
		},
		{
			name: "Error if missing from",
			settings: `{
				"to": "a@b.com",
				"smarthost": "smtp:587"
			}`,
			expectedInitError: "missing from address in email config",
		},
		{
			name: "Minimal valid configuration",
			settings: `{
				"to": "a@b.com",
				"from": "b@b.com",
				"smarthost": "smtp:587"
			}`,
			expectedConfig: Config{
				NotifierConfig: DefaultConfig.NotifierConfig,
				To:             "a@b.com",
				From:           "b@b.com",
				Smarthost:      receivers.HostPort{Host: "smtp", Port: "587"},
				Headers:        map[string]string{},
				HTML:           DefaultConfig.HTML,
			},
		},
		{
			name: "Secret fields from decrypt",
			settings: `{
				"to": "a@b.com",
				"from": "b@b.com",
				"smarthost": "smtp:587"
			}`,
			secrets: map[string][]byte{
				"auth_password": []byte("secret-pass"),
				"auth_secret":   []byte("secret-sec"),
			},
			expectedConfig: Config{
				NotifierConfig: DefaultConfig.NotifierConfig,
				To:             "a@b.com",
				From:           "b@b.com",
				Smarthost:      receivers.HostPort{Host: "smtp", Port: "587"},
				AuthPassword:   "secret-pass",
				AuthSecret:     "secret-sec",
				Headers:        map[string]string{},
				HTML:           DefaultConfig.HTML,
			},
		},
		{
			name: "Secrets override settings",
			settings: `{
				"to": "a@b.com",
				"from": "b@b.com",
				"smarthost": "smtp:587",
				"auth_password": "original"
			}`,
			secrets: map[string][]byte{
				"auth_password": []byte("override"),
			},
			expectedConfig: Config{
				NotifierConfig: DefaultConfig.NotifierConfig,
				To:             "a@b.com",
				From:           "b@b.com",
				Smarthost:      receivers.HostPort{Host: "smtp", Port: "587"},
				AuthPassword:   "override",
				Headers:        map[string]string{},
				HTML:           DefaultConfig.HTML,
			},
		},
		{
			name: "Multiple addresses in to",
			settings: `{
				"to": "a@example.com, ,b@example.com,c@example.com",
				"from": "b@b.com",
				"smarthost": "smtp:587"
			}`,
			expectedConfig: Config{
				NotifierConfig: DefaultConfig.NotifierConfig,
				To:             "a@example.com, ,b@example.com,c@example.com",
				From:           "b@b.com",
				Smarthost:      receivers.HostPort{Host: "smtp", Port: "587"},
				Headers:        map[string]string{},
				HTML:           DefaultConfig.HTML,
			},
		},
		{
			name: "TLS config key from secrets",
			settings: `{
				"to": "a@b.com",
				"from": "b@b.com",
				"smarthost": "smtp:587",
				"tls_config": {"cert": "cert-pem"}
			}`,
			secrets: map[string][]byte{
				"tls_config.key": []byte("tls-private-key"),
			},
			expectedConfig: Config{
				NotifierConfig: DefaultConfig.NotifierConfig,
				To:             "a@b.com",
				From:           "b@b.com",
				Smarthost:      receivers.HostPort{Host: "smtp", Port: "587"},
				Headers:        map[string]string{},
				HTML:           DefaultConfig.HTML,
				TLSConfig: httpcfg.TLSConfig{
					Cert: "cert-pem",
					Key:  commoncfg.Secret("tls-private-key"),
				},
			},
		},
		{
			name: "All secrets at once",
			settings: `{
				"to": "a@b.com",
				"from": "b@b.com",
				"smarthost": "smtp:587",
				"tls_config": {"cert": "cert-pem"}
			}`,
			secrets: map[string][]byte{
				"auth_password":  []byte("pass"),
				"auth_secret":    []byte("sec"),
				"tls_config.key": []byte("key"),
			},
			expectedConfig: Config{
				NotifierConfig: DefaultConfig.NotifierConfig,
				To:             "a@b.com",
				From:           "b@b.com",
				Smarthost:      receivers.HostPort{Host: "smtp", Port: "587"},
				AuthPassword:   "pass",
				AuthSecret:     "sec",
				Headers:        map[string]string{},
				HTML:           DefaultConfig.HTML,
				TLSConfig: httpcfg.TLSConfig{
					Cert: "cert-pem",
					Key:  commoncfg.Secret("key"),
				},
			},
		},
		{
			name: "Duplicate headers",
			settings: `{
				"to": "a@b.com",
				"from": "b@b.com",
				"smarthost": "smtp:587",
				"headers": {"Subject": "a", "subject": "b"}
			}`,
			expectedInitError: "duplicate header",
		},
		{
			name:     "FullValidConfigForTesting is valid",
			settings: FullValidConfigForTesting,
			expectedConfig: func() Config {
				cfg := DefaultConfig
				_ = json.Unmarshal([]byte(FullValidConfigForTesting), &cfg)
				// validate() normalizes headers
				cfg.Headers = map[string]string{"Subject": "test subject"}
				return cfg
			}(),
		},
		{
			name:     "GetFullValidConfig round-trips through JSON",
			settings: func() string { b, _ := json.Marshal(GetFullValidConfig()); return string(b) }(),
			secrets: map[string][]byte{
				"auth_password": []byte(string(GetFullValidConfig().AuthPassword)),
				"auth_secret":   []byte(string(GetFullValidConfig().AuthSecret)),
			},
			expectedConfig: GetFullValidConfig(),
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
			name:        "Missing to",
			mutate:      func(cfg *Config) { cfg.To = "" },
			expectedErr: "missing to address in email config",
		},
		{
			name:        "Missing smarthost",
			mutate:      func(cfg *Config) { cfg.Smarthost = receivers.HostPort{} },
			expectedErr: "missing smarthost in email config",
		},
		{
			name:        "Missing from",
			mutate:      func(cfg *Config) { cfg.From = "" },
			expectedErr: "missing from address in email config",
		},
		{
			name:        "Duplicate headers",
			mutate:      func(cfg *Config) { cfg.Headers = map[string]string{"Subject": "a", "subject": "b"} },
			expectedErr: "duplicate header",
		},
		{
			name:        "Invalid TLS config",
			mutate:      func(cfg *Config) { cfg.TLSConfig = httpcfg.TLSConfig{Key: "key-without-cert"} },
			expectedErr: "invalid tls_config",
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
