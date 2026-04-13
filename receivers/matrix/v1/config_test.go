package v1

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
			name:              "Error if empty JSON object",
			settings:          `{}`,
			expectedInitError: "homeserver URL must be specified",
		},
		{
			name:              "Error if homeserver URL is invalid",
			settings:          `{"homeserverUrl": "not a url"}`,
			expectedInitError: "invalid homeserver URL",
		},
		{
			name:              "Error if access token is missing",
			settings:          `{"homeserverUrl": "https://matrix.example.com", "roomId": "!abc:example.com"}`,
			expectedInitError: "access token must be specified",
		},
		{
			name:              "Error if room ID is missing",
			settings:          `{"homeserverUrl": "https://matrix.example.com", "accessToken": "t"}`,
			expectedInitError: "room ID must be specified",
		},
		{
			name:              "Error if room ID is an alias",
			settings:          `{"homeserverUrl": "https://matrix.example.com", "accessToken": "t", "roomId": "#public:example.com"}`,
			expectedInitError: "room ID must be an internal room ID",
		},
		{
			name:              "Error if message type is invalid",
			settings:          `{"homeserverUrl": "https://matrix.example.com", "accessToken": "t", "roomId": "!abc:example.com", "messageType": "m.image"}`,
			expectedInitError: "invalid message type",
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"homeserverUrl": "https://matrix.example.com", "accessToken": "t", "roomId": "!abc:example.com"}`,
			expectedConfig: Config{
				HomeserverURL: "https://matrix.example.com",
				AccessToken:   "t",
				RoomID:        "!abc:example.com",
				MessageType:   MessageTypeText,
				Title:         templates.DefaultMessageTitleEmbed,
				Message:       templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "Access token from secrets takes precedence",
			settings: `{"homeserverUrl": "https://matrix.example.com", "accessToken": "plaintext", "roomId": "!abc:example.com"}`,
			secrets:  map[string][]byte{"accessToken": []byte("decrypted")},
			expectedConfig: Config{
				HomeserverURL: "https://matrix.example.com",
				AccessToken:   "decrypted",
				RoomID:        "!abc:example.com",
				MessageType:   MessageTypeText,
				Title:         templates.DefaultMessageTitleEmbed,
				Message:       templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "Trailing slash trimmed from homeserver URL",
			settings: `{"homeserverUrl": "https://matrix.example.com/", "accessToken": "t", "roomId": "!abc:example.com"}`,
			expectedConfig: Config{
				HomeserverURL: "https://matrix.example.com",
				AccessToken:   "t",
				RoomID:        "!abc:example.com",
				MessageType:   MessageTypeText,
				Title:         templates.DefaultMessageTitleEmbed,
				Message:       templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				HomeserverURL: "https://matrix.example.com",
				AccessToken:   "test-token",
				RoomID:        "!abc:example.com",
				MessageType:   MessageTypeText,
				Title:         "test-title",
				Message:       "test-message",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			decryptFn := receiversTesting.DecryptForTesting(c.secrets)
			actual, err := NewConfig(json.RawMessage(c.settings), decryptFn)

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
