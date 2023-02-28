package line

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
			expectedInitError: `could not find token in settings`,
		},
		{
			name:              "Error if Token is empty",
			settings:          `{ "token": "" }`,
			expectedInitError: `could not find token in settings`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"token": "test"}`,
			expectedConfig: Config{
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
				Token:       "test",
			},
		},
		{
			name:     "Should override token from secure settings",
			settings: `{"token": "test"}`,
			secureSettings: map[string][]byte{
				"token": []byte("test-token"),
			},
			expectedConfig: Config{
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
				Token:       "test-token",
			},
		},
		{
			name:     "Should set token from secure settings",
			settings: `{}`,
			secureSettings: map[string][]byte{
				"token": []byte("test-token"),
			},
			expectedConfig: Config{
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
				Token:       "test-token",
			},
		},
		{
			name:     "All empty fields = minimal valid configuration",
			settings: `{"token": "", "title": "", "description": "" }`,
			secureSettings: map[string][]byte{
				"token": []byte("test-token"),
			},
			expectedConfig: Config{
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
				Token:       "test-token",
			},
		},
		{
			name:           "Extracts all fields",
			settings:       FullValidConfigForTesting,
			secureSettings: map[string][]byte{},
			expectedConfig: Config{
				Title:       "test-title",
				Description: "test-description",
				Token:       "test",
			},
		},
		{
			name:           "Extracts all fields + override from secrets",
			settings:       FullValidConfigForTesting,
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				Title:       "test-title",
				Description: "test-description",
				Token:       "test-secret-token",
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
