package pushover

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
			expectedInitError: `user key not found`,
		},
		{
			name:              "Error if userKey is missing",
			settings:          `{ "apiToken" : "test-api-token" }`,
			expectedInitError: `user key not found`,
		},
		{
			name:              "Error if apiToken is missing",
			settings:          `{ "userKey": "test-user-key" }`,
			expectedInitError: `API token not found`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"userKey": "test-user-key", "apiToken" : "test-api-token" }`,
			expectedConfig: Config{
				UserKey:          "test-user-key",
				APIToken:         "test-api-token",
				AlertingPriority: 0,
				OkPriority:       0,
				Retry:            0,
				Expire:           0,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           true,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "Minimal valid configuration from secrets",
			settings: `{ }`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedConfig: Config{
				UserKey:          "test-user-key",
				APIToken:         "test-api-token",
				AlertingPriority: 0,
				OkPriority:       0,
				Retry:            0,
				Expire:           0,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           true,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "Should overwrite token from secrets",
			settings: `{"userKey": "test-", "apiToken" : "test-api" }`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedConfig: Config{
				UserKey:          "test-user-key",
				APIToken:         "test-api-token",
				AlertingPriority: 0,
				OkPriority:       0,
				Retry:            0,
				Expire:           0,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           true,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          templates.DefaultMessageEmbed,
			},
		},
		{
			name: "All empty fields = minimal valid configuration",
			settings: `{
				"userKey": "",
				"apiToken": "",
				"priority": "",
				"okPriority": "",
				"retry": "",
				"expire": "",
				"device": "",
				"sound": "",
				"okSound": "",
				"uploadImage": null,
				"title": "",
				"message": ""
			}`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedConfig: Config{
				UserKey:          "test-user-key",
				APIToken:         "test-api-token",
				AlertingPriority: 0,
				OkPriority:       0,
				Retry:            0,
				Expire:           0,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           true,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				UserKey:          "test-user-key",
				APIToken:         "test-api-token",
				AlertingPriority: 1,
				OkPriority:       2,
				Retry:            555,
				Expire:           333,
				Device:           "test-device",
				AlertingSound:    "test-sound",
				OkSound:          "test-ok-sound",
				Upload:           false,
				Title:            "test-title",
				Message:          "test-message",
			},
		},
		{
			name:           "Extracts all fields + override from secrets",
			settings:       FullValidConfigForTesting,
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				UserKey:          "test-secret-user-key",
				APIToken:         "test-secret-api-token",
				AlertingPriority: 1,
				OkPriority:       2,
				Retry:            555,
				Expire:           333,
				Device:           "test-device",
				AlertingSound:    "test-sound",
				OkSound:          "test-ok-sound",
				Upload:           false,
				Title:            "test-title",
				Message:          "test-message",
			},
		},
		{
			name: "Should treat strings as numbers",
			settings: `{
				"priority": "1",
				"okPriority": "2",
				"retry": "555",
				"expire": "333"
			}`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedConfig: Config{
				UserKey:          "test-user-key",
				APIToken:         "test-api-token",
				AlertingPriority: 1,
				OkPriority:       2,
				Retry:            555,
				Expire:           333,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           true,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          templates.DefaultMessageEmbed,
			},
		},
		{
			name: "Should fail if priority is not number",
			settings: `{
				"priority": "test"
			}`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedInitError: `failed to convert alerting priority to integer`,
		},
		{
			name: "Should fail if priority is not integer",
			settings: `{
				"priority": 123.23
			}`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedInitError: `failed to convert alerting priority to integer`,
		},
		{
			name: "Should fail if okPriority is not number",
			settings: `{
				"okPriority": "test-ok"
			}`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedInitError: "failed to convert OK priority to integer",
		},
		{
			name: "Should fail if okPriority is not integer",
			settings: `{
				"okPriority": 123.23
			}`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedInitError: `failed to convert OK priority to integer`,
		},
		{
			name: "Should fallback to 0 if retry is not number",
			settings: `{
				"retry": "test-retry"
			}`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedConfig: Config{
				UserKey:          "test-user-key",
				APIToken:         "test-api-token",
				AlertingPriority: 0,
				OkPriority:       0,
				Retry:            0,
				Expire:           0,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           true,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          templates.DefaultMessageEmbed,
			},
		},
		{
			name: "Should default to 0 if retry is not integer",
			settings: `{
				"retry": 123.44
			}`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedConfig: Config{
				UserKey:          "test-user-key",
				APIToken:         "test-api-token",
				AlertingPriority: 0,
				OkPriority:       0,
				Retry:            0,
				Expire:           0,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           true,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          templates.DefaultMessageEmbed,
			},
		},
		{
			name: "Should fallback to 0 if expire is not number",
			settings: `{
				"expire": "test-expire"
			}`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedConfig: Config{
				UserKey:          "test-user-key",
				APIToken:         "test-api-token",
				AlertingPriority: 0,
				OkPriority:       0,
				Retry:            0,
				Expire:           0,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           true,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          templates.DefaultMessageEmbed,
			},
		},
		{
			name: "Should default to 0 if expire is not integer",
			settings: `{
				"expire": 123.44
			}`,
			secureSettings: map[string][]byte{
				"userKey":  []byte("test-user-key"),
				"apiToken": []byte("test-api-token"),
			},
			expectedConfig: Config{
				UserKey:          "test-user-key",
				APIToken:         "test-api-token",
				AlertingPriority: 0,
				OkPriority:       0,
				Retry:            0,
				Expire:           0,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           true,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          templates.DefaultMessageEmbed,
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
