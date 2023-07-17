package jsm

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
			expectedInitError: `could not find api key property in settings`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"apiKey": "test-api-key" }`,
			expectedConfig: Config{
				APIKey:           "test-api-key",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendTags,
			},
		},
		{
			name:     "Minimal valid configuration from secrets",
			settings: `{ }`,
			secureSettings: map[string][]byte{
				"apiKey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				APIKey:           "test-api-key",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendTags,
			},
		},
		{
			name:     "Should overwrite token from secrets",
			settings: `{ "apiKey": "test" }`,
			secureSettings: map[string][]byte{
				"apiKey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				APIKey:           "test-api-key",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendTags,
			},
		},
		{
			name:     "Send tags as tags",
			settings: `{ "sendTagsAs" : "tags" }`,
			secureSettings: map[string][]byte{
				"apiKey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				APIKey:           "test-api-key",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendTags,
			},
		},
		{
			name:     "Send tags as details",
			settings: `{ "sendTagsAs" : "details" }`,
			secureSettings: map[string][]byte{
				"apiKey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				APIKey:           "test-api-key",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendDetails,
			},
		},
		{
			name:     "Send tags as both",
			settings: `{ "sendTagsAs" : "both" }`,
			secureSettings: map[string][]byte{
				"apiKey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				APIKey:           "test-api-key",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendBoth,
			},
		},
		{
			name:     "Error if send tags is not known",
			settings: `{ "sendTagsAs" : "test-tags" }`,
			secureSettings: map[string][]byte{
				"apiKey": []byte("test-api-key"),
			},
			expectedInitError: `invalid value for sendTagsAs: "test-tags"`,
		},
		{
			name:     "Should use default message if all spaces",
			settings: `{ "message" : " " }`,
			secureSettings: map[string][]byte{
				"apiKey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				APIKey:           "test-api-key",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendTags,
			},
		},
		{
			name: "All empty fields = minimal valid configuration",
			secureSettings: map[string][]byte{
				"apiKey": []byte("test-api-key"),
			},
			settings: `{
				"apiKey": "", 
				"apiUrl" : "", 
				"message" : "", 
				"description": "", 
				"autoClose": null, 
				"overridePriority": null, 
				"sendTagsAs": ""
			}`,
			expectedConfig: Config{
				APIKey:           "test-api-key",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendTags,
			},
		},
		{
			name:           "Extracts all fields",
			secureSettings: map[string][]byte{},
			settings:       FullValidConfigForTesting,
			expectedConfig: Config{
				APIKey:           "test-api-key",
				APIUrl:           "http://localhost",
				Message:          "test-message",
				Description:      "test-description",
				AutoClose:        false,
				OverridePriority: false,
				SendTagsAs:       "both",
			},
		},
		{
			name:           "Extracts all fields + override from secrets",
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			settings:       FullValidConfigForTesting,
			expectedConfig: Config{
				APIKey:           "test-secret-api-key",
				APIUrl:           "http://localhost",
				Message:          "test-message",
				Description:      "test-description",
				AutoClose:        false,
				OverridePriority: false,
				SendTagsAs:       "both",
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
