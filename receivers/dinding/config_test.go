package dinding

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
			expectedInitError: `could not find url property in settings`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"url": "http://localhost"}`,
			expectedConfig: Config{
				URL:         "http://localhost",
				MessageType: defaultDingdingMsgType,
				Title:       templates.DefaultMessageTitleEmbed,
				Message:     templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "All empty fields = minimal valid configuration",
			settings: `{"url": "http://localhost", "message": "", "title": "", "msgType": ""}`,
			expectedConfig: Config{
				URL:         "http://localhost",
				MessageType: defaultDingdingMsgType,
				Title:       templates.DefaultMessageTitleEmbed,
				Message:     templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "All supported fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				URL:         "http://localhost",
				MessageType: "actionCard",
				Title:       "Alerts firing: {{ len .Alerts.Firing }}",
				Message:     "{{ len .Alerts.Firing }} alerts are firing, {{ len .Alerts.Resolved }} are resolved",
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
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
