package victorops

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
			expectedInitError: `could not find victorops url property in settings`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"url": "http://localhost" }`,
			expectedConfig: Config{
				URL:         "http://localhost",
				MessageType: DefaultMessageType,
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "Minimal valid configuration with secrets",
			settings: `{}`,
			secureSettings: map[string][]byte{
				"url": []byte("http://localhost-secret"),
			},
			expectedConfig: Config{
				URL:         "http://localhost-secret",
				MessageType: DefaultMessageType,
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
			},
		},
		{
			name: "All empty fields = minimal valid configuration",
			settings: `{
				"url" : "http://localhost",
				"messageType" :"",
				"title" :"",
				"description" :""
			}`,
			expectedConfig: Config{
				URL:         "http://localhost",
				MessageType: DefaultMessageType,
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				URL:         "http://localhost",
				MessageType: "test-messagetype",
				Title:       "test-title",
				Description: "test-description",
			},
		},
		{
			name:           "Should overwrite from secrets",
			settings:       FullValidConfigForTesting,
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				URL:         "http://localhost-secret",
				MessageType: "test-messagetype",
				Title:       "test-title",
				Description: "test-description",
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
