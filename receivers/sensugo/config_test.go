package sensugo

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
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
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
		{
			name:           "Extracts all fields + override from encrypted",
			settings:       FullValidConfigForTesting,
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				URL:       "http://localhost",
				Entity:    "test-entity",
				Check:     "test-check",
				Namespace: "test-namespace",
				Handler:   "test-handler",
				APIKey:    "test-secret-api-key",
				Message:   "test-message",
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
