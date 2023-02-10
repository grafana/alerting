package teams

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

func TestValidateConfig(t *testing.T) {
	cases := []struct {
		name              string
		settings          string
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
			name:     "Minimal valid configuration",
			settings: `{"url": "http://localhost" }`,
			expectedConfig: Config{
				URL:          "http://localhost",
				Message:      `{{ template "teams.default.message" .}}`,
				Title:        templates.DefaultMessageTitleEmbed,
				SectionTitle: "",
			},
		},
		{
			name: "All empty fields = minimal valid configuration",
			settings: `{
				"url": "http://localhost",  
				"message" : "",
				"title" : "",
				"sectiontitle" : ""
			}`,
			expectedConfig: Config{
				URL:          "http://localhost",
				Message:      `{{ template "teams.default.message" .}}`,
				Title:        templates.DefaultMessageTitleEmbed,
				SectionTitle: "",
			},
		},
		{
			name: "Extracts all fields",
			settings: `{
				"url": "http://localhost",  
				"message" : "test-message",
				"title" : "test-title",
				"sectiontitle" : "test-second-title"
			}`,
			expectedConfig: Config{
				URL:          "http://localhost",
				Message:      `test-message`,
				Title:        "test-title",
				SectionTitle: "test-second-title",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &receivers.NotificationChannelConfig{
				Settings: json.RawMessage(c.settings),
			}
			fc, err := receiversTesting.NewFactoryConfigForValidateConfigTesting(t, m)
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
