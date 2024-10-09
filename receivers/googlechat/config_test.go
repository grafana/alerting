package googlechat

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
			expectedInitError: `could not find url property in settings`,
		},
		{
			name:              "Error if URL is empty",
			settings:          `{ "url": "" }`,
			secrets:           map[string][]byte{},
			expectedInitError: `could not find url property in settings`,
		},
		{
			name:     "Minimum valid configuration with url in plain text",
			settings: `{ "url": "http://localhost" }`,
			secrets:  map[string][]byte{},
			expectedConfig: Config{
				Title:   templates.DefaultMessageTitleEmbed,
				Message: templates.DefaultMessageEmbed,
				URL:     "http://localhost",
			},
		},
		{
			name:     "Minimum valid configuration with url in secrets",
			settings: `{ "url": "" }`,
			secrets: map[string][]byte{
				"url": []byte("http://localhost"),
			},
			expectedConfig: Config{
				Title:   templates.DefaultMessageTitleEmbed,
				Message: templates.DefaultMessageEmbed,
				URL:     "http://localhost",
			},
		},
		{
			name:     "Should overwrite url from secrets",
			settings: `{ "url": "http://localhost" }`,
			secrets: map[string][]byte{
				"url": []byte("http://test"),
			},
			expectedConfig: Config{
				Title:   templates.DefaultMessageTitleEmbed,
				Message: templates.DefaultMessageEmbed,
				URL:     "http://test",
			},
		},
		{
			name:     "All empty fields = minimal valid configuration",
			settings: `{"url": "http://localhost", "title": "", "message": "", "avatar_url" : "", "use_discord_username": null}`,
			expectedConfig: Config{
				Title:   templates.DefaultMessageTitleEmbed,
				Message: templates.DefaultMessageEmbed,
				URL:     "http://localhost",
			},
		},
		{
			name:     "Extracts all fields + override from secrets",
			settings: FullValidConfigForTesting,
			secrets:  receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				Title:   "test-title",
				Message: "test-message",
				URL:     "http://localhost/url-secret",
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
