package email

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/templates"
)

func TestNewConfig(t *testing.T) {
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
			expectedInitError: `could not find addresses in settings`,
		},
		{
			name:              "Error if URL is empty",
			settings:          `{ "addresses": "" }`,
			expectedInitError: `could not find addresses in settings`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"addresses": "test@grafana.com"}`,
			expectedConfig: Config{
				SingleEmail: false,
				Addresses: []string{
					"test@grafana.com",
				},
				Message: "",
				Subject: templates.DefaultMessageTitleEmbed,
			},
		},
		{
			name:     "Multiple addresses with different delimiters",
			settings: `{"addresses": "test@grafana.com,test2@grafana.com;test3@grafana.com\ntest4@granafa.com"}`,
			expectedConfig: Config{
				SingleEmail: false,
				Addresses: []string{
					"test@grafana.com",
					"test2@grafana.com",
					"test3@grafana.com",
					"test4@granafa.com",
				},
				Message: "",
				Subject: templates.DefaultMessageTitleEmbed,
			},
		},
		{
			name:     "All empty fields = minimal valid configuration",
			settings: `{"addresses": "test@grafana.com", "subject": "", "message": "", "singleEmail": null}`,
			expectedConfig: Config{
				SingleEmail: false,
				Addresses: []string{
					"test@grafana.com",
				},
				Message: "",
				Subject: templates.DefaultMessageTitleEmbed,
			},
		},
		{
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				SingleEmail: true,
				Addresses: []string{
					"test@grafana.com",
				},
				Message: "test-message",
				Subject: "test-subject",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := NewConfig(json.RawMessage(c.settings))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
