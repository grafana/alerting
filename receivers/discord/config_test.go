package discord

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
			expectedInitError: `failed to unmarshal settings`,
		},
		{
			name:              "Error if empty JSON object",
			settings:          `{}`,
			expectedInitError: `could not find webhook url property in settings`,
		},
		{
			name:              "Error if URL is empty",
			settings:          `{ "url": "" }`,
			expectedInitError: `could not find webhook url property in settings`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"url": "http://localhost"}`,
			expectedConfig: Config{
				Title:              templates.DefaultMessageTitleEmbed,
				Message:            templates.DefaultMessageEmbed,
				AvatarURL:          "",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: false,
			},
		},
		{
			name:     "Minimal valid configuration from secure settings",
			settings: `{}`,
			secrets: map[string][]byte{
				"url": []byte("http://localhost"),
			},
			expectedConfig: Config{
				Title:              templates.DefaultMessageTitleEmbed,
				Message:            templates.DefaultMessageEmbed,
				AvatarURL:          "",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: false,
			},
		},
		{
			name:     "All empty fields = minimal valid configuration",
			settings: `{"url": "http://localhost", "title": "", "message": "", "avatar_url" : "", "use_discord_username": null}`,
			expectedConfig: Config{
				Title:              templates.DefaultMessageTitleEmbed,
				Message:            templates.DefaultMessageEmbed,
				AvatarURL:          "",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: false,
			},
		},
		{
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				Title:              "test-title",
				Message:            "test-message",
				AvatarURL:          "http://avatar",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: true,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(c.secrets))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
