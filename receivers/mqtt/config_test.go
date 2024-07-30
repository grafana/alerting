package mqtt

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

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
			name:              "Error if the broker URL does not have the scheme",
			settings:          `{ "brokerUrl" : "localhost" }`,
			expectedInitError: `Invalid MQTT broker URL: Invalid scheme, must be 'tcp' or 'ssl'`,
		},
		{
			name:              "Error if the broker URL has invalid scheme",
			settings:          `{ "brokerUrl" : "http://localhost" }`,
			expectedInitError: `Invalid MQTT broker URL: Invalid scheme, must be 'tcp' or 'ssl'`,
		},
		{
			name:              "Error if the broker URL does not have the port",
			settings:          `{ "brokerUrl" : "tcp://localhost" }`,
			expectedInitError: `Invalid MQTT broker URL: Port must be specified`,
		},
		{
			name:              "Error if the broker URL port is invalid",
			settings:          `{ "brokerUrl" : "tcp://localhost:100000" }`,
			expectedInitError: `Invalid MQTT broker URL: Port must be a valid number between 1 and 65535`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{ "brokerUrl" : "tcp://localhost:1883", "topic": "grafana/alerts"}`,
			expectedConfig: Config{
				Message:   templates.DefaultMessageEmbed,
				BrokerURL: "tcp://localhost:1883",
				Topic:     "grafana/alerts",
				ClientID:  "Grafana",
			},
		},
		{
			name:     "Configuration with insecureSkipVerify",
			settings: `{ "brokerUrl" : "tcp://localhost:1883", "topic": "grafana/alerts", "insecureSkipVerify": true}`,
			expectedConfig: Config{
				Message:            templates.DefaultMessageEmbed,
				BrokerURL:          "tcp://localhost:1883",
				Topic:              "grafana/alerts",
				ClientID:           "Grafana",
				InsecureSkipVerify: true,
			},
		},
		{
			name:     "Minimal valid configuration with secrets",
			settings: `{ "brokerUrl" : "tcp://localhost:1883", "topic": "grafana/alerts", "username": "grafana"}`,
			secureSettings: map[string][]byte{
				"password": []byte("testpasswd"),
			},
			expectedConfig: Config{
				Message:   templates.DefaultMessageEmbed,
				BrokerURL: "tcp://localhost:1883",
				Topic:     "grafana/alerts",
				ClientID:  "Grafana",
				Username:  "grafana",
				Password:  "testpasswd",
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
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
