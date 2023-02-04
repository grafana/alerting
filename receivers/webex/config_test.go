package webex

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
			name: "Extracts all fields",
			settings: `{
				"message" :"test-message",  
				"room_id" :"test-room-id",
				"api_url" :"http://localhost",
				"bot_token" :"12345"
			}`,
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
			require.Equal(t, c.expectedConfig, *actual)
		})
	}
}
