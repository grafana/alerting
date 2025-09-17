package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

func TestNewConfig(t *testing.T) {
	cases := []struct {
		name              string
		settings          string
		secureSettings    map[string][]byte
		expectedConfig    Config
		expectedInitError string
	}{
		{
			name:              "Error if empty",
			settings:          "",
			expectedInitError: `failed to unmarshal settings`,
		},
		{
			name:              "Error if broker URL is missing",
			settings:          `{}`,
			expectedInitError: `MQTT broker URL must be specified`,
		},
		{
			name:              "Error if topic is missing",
			settings:          `{ "brokerUrl" : "tcp://localhost:1883" }`,
			expectedInitError: `MQTT topic must be specified`,
		},
		{
			name:              "Invalid message format",
			settings:          `{ "brokerUrl" : "tcp://localhost:1883", "topic": "grafana/alerts", "messageFormat": "invalid"}`,
			expectedInitError: `Invalid message format, must be 'json' or 'text'`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{ "brokerUrl" : "tcp://localhost:1883", "topic": "grafana/alerts"}`,
			expectedConfig: Config{
				Message:       templates.DefaultMessageEmbed,
				BrokerURL:     "tcp://localhost:1883",
				Topic:         "grafana/alerts",
				MessageFormat: MessageFormatJSON,
				TLSConfig: &receivers.TLSConfig{
					ServerName: "localhost",
				},
			},
		},
		{
			name:     "Configuration with insecureSkipVerify",
			settings: `{ "brokerUrl" : "tcp://localhost:1883", "topic": "grafana/alerts", "tlsConfig": {"insecureSkipVerify": true}}`,
			expectedConfig: Config{
				Message:       templates.DefaultMessageEmbed,
				BrokerURL:     "tcp://localhost:1883",
				Topic:         "grafana/alerts",
				MessageFormat: MessageFormatJSON,
				TLSConfig: &receivers.TLSConfig{
					InsecureSkipVerify: true,
					ServerName:         "localhost",
				},
			},
		},
		{
			name:     "Configuration with a client ID",
			settings: `{ "brokerUrl" : "tcp://localhost:1883", "topic": "grafana/alerts", "clientId": "test-client-id"}`,
			expectedConfig: Config{
				Message:       templates.DefaultMessageEmbed,
				BrokerURL:     "tcp://localhost:1883",
				Topic:         "grafana/alerts",
				MessageFormat: MessageFormatJSON,
				ClientID:      "test-client-id",
				TLSConfig: &receivers.TLSConfig{
					ServerName: "localhost",
				},
			},
		},
		{
			name:     "Minimal valid configuration with secrets",
			settings: `{ "brokerUrl" : "tcp://localhost:1883", "topic": "grafana/alerts", "username": "grafana"}`,
			secureSettings: map[string][]byte{
				"password": []byte("testpasswd"),
			},
			expectedConfig: Config{
				Message:       templates.DefaultMessageEmbed,
				BrokerURL:     "tcp://localhost:1883",
				Topic:         "grafana/alerts",
				MessageFormat: MessageFormatJSON,
				Username:      "grafana",
				Password:      "testpasswd",
				TLSConfig: &receivers.TLSConfig{
					ServerName: "localhost",
				},
			},
		},
		{
			name:     "Configuration with tlsConfig",
			settings: `{ "brokerUrl" : "tcp://localhost:1883", "topic": "grafana/alerts"}`,
			secureSettings: map[string][]byte{
				"tlsConfig.caCertificate":     []byte("test-ca-cert"),
				"tlsConfig.clientCertificate": []byte("test-client-cert"),
				"tlsConfig.clientKey":         []byte("test-client-key"),
			},
			expectedConfig: Config{
				Message:       templates.DefaultMessageEmbed,
				BrokerURL:     "tcp://localhost:1883",
				Topic:         "grafana/alerts",
				MessageFormat: MessageFormatJSON,
				TLSConfig: &receivers.TLSConfig{
					InsecureSkipVerify: false,
					ServerName:         "localhost",
					CACertificate:      "test-ca-cert",
					ClientKey:          "test-client-key",
					ClientCertificate:  "test-client-cert",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(c.secureSettings))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}

			if c.expectedConfig.ClientID == "" {
				require.Regexp(t, `grafana_\d+`, actual.ClientID)
				actual.ClientID = ""
			}

			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
