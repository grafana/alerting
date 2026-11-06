package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alerting/receivers"

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
			name:              "Null addresses",
			settings:          `{"addresses":null}`,
			expectedInitError: `could not find addresses in settings`,
		},
		{
			name:           "Delimiter-only addresses remain accepted",
			settings:       `{"addresses":",;\n"}`,
			expectedConfig: Config{Addresses: receivers.DelimitedStrings{}, Subject: templates.DefaultMessageTitleEmbed},
		},
		{
			name:           "Whitespace-only entries are skipped and addresses are trimmed",
			settings:       `{"addresses":" a@example.com ;\tb@example.com\r\n ;\u00a0c@example.com\u2003"}`,
			expectedConfig: Config{Addresses: receivers.DelimitedStrings{"a@example.com", "b@example.com", "c@example.com"}, Subject: templates.DefaultMessageTitleEmbed},
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
			actual, err := NewConfig(json.RawMessage(c.settings), nil)

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)

			encoded, err := json.Marshal(actual)
			require.NoError(t, err)
			roundTrip, err := NewConfig(encoded, nil)
			require.NoError(t, err)
			require.Equal(t, actual, roundTrip)

			encoded, err = yaml.Marshal(actual)
			require.NoError(t, err)
			var yamlConfig Config
			require.NoError(t, yaml.Unmarshal(encoded, &yamlConfig))
			require.Equal(t, actual, yamlConfig)
		})
	}
}
