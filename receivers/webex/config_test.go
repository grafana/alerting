package webex

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
			name:     "Minimal valid configuration",
			settings: `{}`,
			expectedConfig: Config{
				Message: templates.DefaultMessageEmbed,
				RoomID:  "",
				APIURL:  DefaultAPIURL,
				Token:   "",
			},
		},
		{
			name:              "Error if url is not valid",
			settings:          `{ "api_url" : "ostgres://user:abc{DEf1=ghi@example.com:5432/db?sslmode=require" }`,
			expectedInitError: `invalid URL "ostgres://user:abc{DEf1=ghi@example.com:5432/db?sslmode=require"`,
		},
		{
			name: "All empty fields = minimal valid configuration",
			settings: `{
				"message" :"",  
				"room_id" :"",
				"api_url" :"",
				"bot_token" :""
			}`,
			expectedConfig: Config{
				Message: templates.DefaultMessageEmbed,
				RoomID:  "",
				APIURL:  DefaultAPIURL,
				Token:   "",
			},
		},
		{
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				Message: "test-message",
				RoomID:  "test-room-id",
				APIURL:  "http://localhost",
				Token:   "12345",
			},
		},
		{
			name:     "Extracts token from secrets",
			settings: `{}`,
			secureSettings: map[string][]byte{
				"bot_token": []byte("test-token"),
			},
			expectedConfig: Config{
				Message: templates.DefaultMessageEmbed,
				RoomID:  "",
				APIURL:  DefaultAPIURL,
				Token:   "test-token",
			},
		},
		{
			name:     "Overrides token from secrets",
			settings: `{ "bot_token": "12345" }`,
			secureSettings: map[string][]byte{
				"bot_token": []byte("test-token"),
			},
			expectedConfig: Config{
				Message: templates.DefaultMessageEmbed,
				RoomID:  "",
				APIURL:  DefaultAPIURL,
				Token:   "test-token",
			},
		},
		{
			name:           "Extracts all fields + override from secrets",
			settings:       FullValidConfigForTesting,
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				Message: "test-message",
				RoomID:  "test-room-id",
				APIURL:  "http://localhost",
				Token:   "12345-secret",
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
