package telegram

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
			expectedInitError: `failed to unmarshal settings: unexpected end of JSON input`,
		},
		{
			name:              "Error if empty JSON object",
			settings:          `{}`,
			expectedInitError: `could not find Bot Token in settings`,
		},
		{
			name:              "Error if bottoken is missing",
			settings:          `{ "chatid" : "test-chat-id" }`,
			expectedInitError: `could not find Bot Token in settings`,
		},
		{
			name:              "Error if chatid is missing",
			settings:          `{ "bottoken" : "12345" }`,
			expectedInitError: `could not find Chat Id in settings`,
		},

		{
			name:     "Minimal valid configuration",
			settings: `{ "bottoken": "test-token", "chatid": "test-chat-id" }`,
			expectedConfig: Config{
				BotToken:             "test-token",
				ChatID:               "test-chat-id",
				Message:              templates.DefaultMessageEmbed,
				ParseMode:            DefaultTelegramParseMode,
				DisableNotifications: false,
			},
		},
		{
			name:     "Minimal valid configuration from secrets",
			settings: `{"chatid": "test-chat-id" }`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token"),
			},
			expectedConfig: Config{
				BotToken:             "test-token",
				ChatID:               "test-chat-id",
				Message:              templates.DefaultMessageEmbed,
				ParseMode:            DefaultTelegramParseMode,
				DisableNotifications: false,
			},
		},
		{
			name:     "Should overwrite token from secrets",
			settings: `{"bottoken": "token", "chatid" : "test-chat-id" }`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token-key"),
			},
			expectedConfig: Config{
				BotToken:             "test-token-key",
				ChatID:               "test-chat-id",
				Message:              templates.DefaultMessageEmbed,
				ParseMode:            DefaultTelegramParseMode,
				DisableNotifications: false,
			},
		},
		{
			name: "All empty fields = minimal valid configuration",
			settings: `{
				"chatid" :"chat-id",
				"message" :"",
				"parse_mode" :"",
				"disable_notifications" : null
			}`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token"),
			},
			expectedConfig: Config{
				BotToken:             "test-token",
				ChatID:               "chat-id",
				Message:              templates.DefaultMessageEmbed,
				ParseMode:            DefaultTelegramParseMode,
				DisableNotifications: false,
			},
		},
		{
			name: "Extracts all fields",
			settings: `{
				"bottoken" :"test-token",
				"chatid" :"12345678",
				"message" :"test-message",
				"parse_mode" :"html",
				"disable_notifications" :true
			}`,
			expectedConfig: Config{
				BotToken:             "test-token",
				ChatID:               "12345678",
				Message:              "test-message",
				ParseMode:            "HTML",
				DisableNotifications: true,
			},
		},
		{
			name:     "should fail if parse mode not supported",
			settings: `{"chatid": "12345678", "parse_mode": "test" }`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token"),
			},
			expectedInitError: "unknown parse_mode",
		},
		{
			name:     "should parse parse_mode (Markdown)",
			settings: `{"chatid": "12345678", "parse_mode": "markdown" }`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token"),
			},
			expectedConfig: Config{
				BotToken:             "test-token",
				ChatID:               "12345678",
				Message:              templates.DefaultMessageEmbed,
				ParseMode:            "Markdown",
				DisableNotifications: false,
			},
		},
		{
			name:     "should parse parse_mode (MarkdownV2)",
			settings: `{"chatid": "12345678", "parse_mode": "markdownv2" }`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token"),
			},
			expectedConfig: Config{
				BotToken:             "test-token",
				ChatID:               "12345678",
				Message:              templates.DefaultMessageEmbed,
				ParseMode:            "MarkdownV2",
				DisableNotifications: false,
			},
		},
		{
			name:     "should parse parse_mode (None)",
			settings: `{"chatid": "12345678", "parse_mode": "None" }`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token"),
			},
			expectedConfig: Config{
				BotToken:             "test-token",
				ChatID:               "12345678",
				Message:              templates.DefaultMessageEmbed,
				ParseMode:            "",
				DisableNotifications: false,
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
