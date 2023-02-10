package sensugo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
	testing2 "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

func TestValidateConfig(t *testing.T) {
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
			name:              "Error if empty JSON object",
			settings:          `{}`,
			expectedInitError: `could not find URL property in settings`,
		},
		{
			name:              "Error if url is missing",
			settings:          `{ "apikey" : "test-api-key" }`,
			expectedInitError: `could not find URL property in settings`,
		},
		{
			name:              "Error if apikey is missing",
			settings:          `{ "url": "http://localhost" }`,
			expectedInitError: `could not find the API key property in settings`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"url": "http://localhost", "apikey" : "test-api-key" }`,
			expectedConfig: Config{
				URL:       "http://localhost",
				Entity:    "",
				Check:     "",
				Namespace: "",
				Handler:   "",
				APIKey:    "test-api-key",
				Message:   templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "Minimal valid configuration from secrets",
			settings: `{"url": "http://localhost" }`,
			secureSettings: map[string][]byte{
				"apikey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				URL:       "http://localhost",
				Entity:    "",
				Check:     "",
				Namespace: "",
				Handler:   "",
				APIKey:    "test-api-key",
				Message:   templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "Should overwrite token from secrets",
			settings: `{"url": "http://localhost", "apikey" : "test" }`,
			secureSettings: map[string][]byte{
				"apikey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				URL:       "http://localhost",
				Entity:    "",
				Check:     "",
				Namespace: "",
				Handler:   "",
				APIKey:    "test-api-key",
				Message:   templates.DefaultMessageEmbed,
			},
		},
		{
			name: "All empty fields = minimal valid configuration",
			settings: `{
				"url": "http://localhost",  
				"entity" : "",
				"check" : "",
				"namespace" : "",
				"handler" : "",
				"apikey" : "",
				"message" : ""
			}`,
			secureSettings: map[string][]byte{
				"apikey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				URL:       "http://localhost",
				Entity:    "",
				Check:     "",
				Namespace: "",
				Handler:   "",
				APIKey:    "test-api-key",
				Message:   templates.DefaultMessageEmbed,
			},
		},
		{
			name: "Extracts all fields",
			settings: `{
				"url": "http://localhost",  
				"entity" : "test-entity",
				"check" : "test-check",
				"namespace" : "test-namespace",
				"handler" : "test-handler",
				"message" : "test-message"
			}`,
			secureSettings: map[string][]byte{
				"apikey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				URL:       "http://localhost",
				Entity:    "test-entity",
				Check:     "test-check",
				Namespace: "test-namespace",
				Handler:   "test-handler",
				APIKey:    "test-api-key",
				Message:   "test-message",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &receivers.NotificationChannelConfig{
				Settings:       json.RawMessage(c.settings),
				SecureSettings: c.secureSettings,
			}
			fc, err := testing2.NewFactoryConfigForValidateConfigTesting(t, m)
			require.NoError(t, err)

			actual, err := ValidateConfig(fc)

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
