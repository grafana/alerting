package dooray

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
			expectedInitError: `could not find url in settings`,
		},
		{
			name:              "Error if Token is empty",
			settings:          `{ "url": "" }`,
			expectedInitError: `could not find url in settings`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"url": "test"}`,
			expectedConfig: Config{
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
				Url:         "test",
			},
		},
		{
			name:     "Should override url from secure settings",
			settings: `{"url": "test"}`,
			secureSettings: map[string][]byte{
				"url": []byte("test-url"),
			},
			expectedConfig: Config{
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
				Url:         "test-url",
			},
		},
		{
			name:     "Should set url from secure settings",
			settings: `{}`,
			secureSettings: map[string][]byte{
				"url": []byte("test-url"),
			},
			expectedConfig: Config{
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
				Url:         "test-url",
			},
		},
		{
			name:     "All empty fields = minimal valid configuration",
			settings: `{"url": "", "title": "", "description": "" }`,
			secureSettings: map[string][]byte{
				"url": []byte("test-url"),
			},
			expectedConfig: Config{
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
				Url:         "test-url",
			},
		},
		{
			name:           "Extracts all fields",
			settings:       FullValidConfigForTesting,
			secureSettings: map[string][]byte{},
			expectedConfig: Config{
				Title:       "test-title",
				Description: "test-description",
				Url:         "test",
			},
		},
		{
			name:           "Extracts all fields + override from secrets",
			settings:       FullValidConfigForTesting,
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				Title:       "test-title",
				Description: "test-description",
				Url:         "test-secret-url",
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
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
