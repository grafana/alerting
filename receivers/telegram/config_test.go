package telegram

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
				BotToken:              "test-token",
				ChatID:                "test-chat-id",
				Message:               templates.DefaultMessageEmbed,
				ParseMode:             DefaultTelegramParseMode,
				DisableWebPagePreview: false,
				ProtectContent:        false,
				DisableNotifications:  false,
			},
		},
		{
			name:     "Minimal valid configuration from secrets",
			settings: `{"chatid": "test-chat-id" }`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token"),
			},
			expectedConfig: Config{
				BotToken:              "test-token",
				ChatID:                "test-chat-id",
				Message:               templates.DefaultMessageEmbed,
				ParseMode:             DefaultTelegramParseMode,
				DisableWebPagePreview: false,
				ProtectContent:        false,
				DisableNotifications:  false,
			},
		},
		{
			name:     "Should overwrite token from secrets",
			settings: `{"bottoken": "token", "chatid" : "test-chat-id" }`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token-key"),
			},
			expectedConfig: Config{
				BotToken:              "test-token-key",
				ChatID:                "test-chat-id",
				Message:               templates.DefaultMessageEmbed,
				ParseMode:             DefaultTelegramParseMode,
				DisableWebPagePreview: false,
				ProtectContent:        false,
				DisableNotifications:  false,
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
				BotToken:              "test-token",
				ChatID:                "chat-id",
				Message:               templates.DefaultMessageEmbed,
				ParseMode:             DefaultTelegramParseMode,
				DisableWebPagePreview: false,
				ProtectContent:        false,
				DisableNotifications:  false,
			},
		},
		{
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				BotToken:              "test-token",
				ChatID:                "12345678",
				Message:               "test-message",
				MessageThreadID:       "13579",
				ParseMode:             "HTML",
				DisableWebPagePreview: true,
				ProtectContent:        true,
				DisableNotifications:  true,
			},
		},
		{
			name:           "Extracts all fields + override from secrets",
			settings:       FullValidConfigForTesting,
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				BotToken:              "test-secret-token",
				ChatID:                "12345678",
				Message:               "test-message",
				MessageThreadID:       "13579",
				ParseMode:             "HTML",
				DisableWebPagePreview: true,
				ProtectContent:        true,
				DisableNotifications:  true,
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
				BotToken:              "test-token",
				ChatID:                "12345678",
				Message:               templates.DefaultMessageEmbed,
				ParseMode:             "Markdown",
				DisableWebPagePreview: false,
				ProtectContent:        false,
				DisableNotifications:  false,
			},
		},
		{
			name:     "should parse parse_mode (MarkdownV2)",
			settings: `{"chatid": "12345678", "parse_mode": "markdownv2" }`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token"),
			},
			expectedConfig: Config{
				BotToken:              "test-token",
				ChatID:                "12345678",
				Message:               templates.DefaultMessageEmbed,
				ParseMode:             "MarkdownV2",
				DisableWebPagePreview: false,
				ProtectContent:        false,
				DisableNotifications:  false,
			},
		},
		{
			name:     "should parse parse_mode (None)",
			settings: `{"chatid": "12345678", "parse_mode": "None" }`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token"),
			},
			expectedConfig: Config{
				BotToken:              "test-token",
				ChatID:                "12345678",
				Message:               templates.DefaultMessageEmbed,
				ParseMode:             "",
				DisableWebPagePreview: false,
				ProtectContent:        false,
				DisableNotifications:  false,
			},
		},
		{
			name:     "should fail if message_thread_id is not an int",
			settings: `{"chatid": "12345678", "message_thread_id": "notanint"}`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token"),
			},
			expectedInitError: "message thread id must be an integer",
		},
		{
			name:     "should fail if message_thread_id is not a valid int32",
			settings: `{"chatid": "12345678", "message_thread_id": "21474836471"}`,
			secureSettings: map[string][]byte{
				"bottoken": []byte("test-token"),
			},
			expectedInitError: "message thread id must be an int32",
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
