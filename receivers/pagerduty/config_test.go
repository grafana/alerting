package pagerduty

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

func TestNewConfig(t *testing.T) {
	hostName := "Grafana-TEST-host"
	provideHostName := func() (string, error) {
		return hostName, nil
	}
	original := getHostname
	t.Cleanup(func() {
		getHostname = original
	})
	getHostname = provideHostName

	cases := []struct {
		name              string
		settings          string
		secureSettings    map[string][]byte
		expectedConfig    Config
		expectedInitError string
		hostnameOverride  func() (string, error)
	}{
		{
			name:              "Error if empty",
			settings:          "",
			expectedInitError: `failed to unmarshal settings`,
		},
		{
			name:              "Error if empty JSON object",
			settings:          `{}`,
			expectedInitError: `could not find integration key property in settings`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"integrationKey": "test-api-key" }`,
			expectedConfig: Config{
				Key:       "test-api-key",
				Severity:  DefaultSeverity,
				Details:   defaultDetails,
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    hostName,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
		},
		{
			name:     "Minimal valid configuration",
			settings: `{}`,
			secureSettings: map[string][]byte{
				"integrationKey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				Key:       "test-api-key",
				Severity:  DefaultSeverity,
				Details:   defaultDetails,
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    hostName,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
		},
		{
			name:     "Should overwrite token from secrets",
			settings: `{ "integrationKey": "test" }`,
			secureSettings: map[string][]byte{
				"integrationKey": []byte("test-api-key"),
			},
			expectedConfig: Config{
				Key:       "test-api-key",
				Severity:  DefaultSeverity,
				Details:   defaultDetails,
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    hostName,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
		},
		{
			name: "All empty fields = minimal valid configuration",
			secureSettings: map[string][]byte{
				"integrationKey": []byte("test-api-key"),
			},
			settings: `{
				"integrationKey": "", 
				"severity" : "", 
				"class" : "", 
				"component": "", 
				"group": "", 
				"summary": "", 
				"source": "",
				"client" : "",
				"client_url": ""
			}`,
			expectedConfig: Config{
				Key:       "test-api-key",
				Severity:  DefaultSeverity,
				Details:   defaultDetails,
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    hostName,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
		},
		{
			name:     "Extract all fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				Key:       "test-api-key",
				Severity:  "test-severity",
				Details:   defaultDetails,
				Class:     "test-class",
				Component: "test-component",
				Group:     "test-group",
				Summary:   "test-summary",
				Source:    "test-source",
				Client:    "test-client",
				ClientURL: "test-client-url",
				URL:       "test-api-url",
			},
		},
		{
			name:           "Extract all fields + override from secrets",
			settings:       FullValidConfigForTesting,
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				Key:       "test-secret-api-key",
				Severity:  "test-severity",
				Details:   defaultDetails,
				Class:     "test-class",
				Component: "test-component",
				Group:     "test-group",
				Summary:   "test-summary",
				Source:    "test-source",
				Client:    "test-client",
				ClientURL: "test-client-url",
				URL:       "test-api-url",
			},
		},
		{
			name: "Should merge default details with user-defined ones",
			secureSettings: map[string][]byte{
				"integrationKey": []byte("test-api-key"),
			},
			settings: `{
				"details" : {
					"test-field" : "test",
					"test-field-2": "test-2"
				}
			}`,
			expectedConfig: Config{
				Key:      "test-api-key",
				Severity: DefaultSeverity,
				Details: map[string]string{
					"firing":       `{{ template "__text_alert_list" .Alerts.Firing }}`,
					"resolved":     `{{ template "__text_alert_list" .Alerts.Resolved }}`,
					"num_firing":   `{{ .Alerts.Firing | len }}`,
					"num_resolved": `{{ .Alerts.Resolved | len }}`,
					"test-field":   "test",
					"test-field-2": "test-2",
				},
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    hostName,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
		},
		{
			name: "Should overwrite default details with user-defined ones when keys are duplicated",
			secureSettings: map[string][]byte{
				"integrationKey": []byte("test-api-key"),
			},
			settings: `{
				"details" : {
					"firing" : "test",
					"resolved": "maybe",
					"num_firing": "a lot",
					"num_resolved": "just a few"
				}
			}`,
			expectedConfig: Config{
				Key:      "test-api-key",
				Severity: DefaultSeverity,
				Details: map[string]string{
					"firing":       "test",
					"resolved":     "maybe",
					"num_firing":   "a lot",
					"num_resolved": "just a few",
				},
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    hostName,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
		},
		{
			name: "Custom details should be case-sensitive",
			secureSettings: map[string][]byte{
				"integrationKey": []byte("test-api-key"),
			},
			settings: `{
				"details" : {
					"Firing" : "test",
					"Resolved": "maybe",
					"nuM_firing": "a lot",
					"num_reSolved": "just a few"
				}
			}`,
			expectedConfig: Config{
				Key:      "test-api-key",
				Severity: DefaultSeverity,
				Details: map[string]string{
					"firing":       `{{ template "__text_alert_list" .Alerts.Firing }}`,
					"resolved":     `{{ template "__text_alert_list" .Alerts.Resolved }}`,
					"num_firing":   `{{ .Alerts.Firing | len }}`,
					"num_resolved": `{{ .Alerts.Resolved | len }}`,
					"Firing":       "test",
					"Resolved":     "maybe",
					"nuM_firing":   "a lot",
					"num_reSolved": "just a few",
				},
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    hostName,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
		},
		{
			name: "Source should fallback to client if hostname cannot be resolved",
			secureSettings: map[string][]byte{
				"integrationKey": []byte("test-api-key"),
			},
			settings: `{
				"client" : "test-client"
			}`,
			hostnameOverride: func() (string, error) {
				return "", errors.New("test")
			},
			expectedConfig: Config{
				Key:       "test-api-key",
				Severity:  DefaultSeverity,
				Details:   defaultDetails,
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    "test-client",
				Client:    "test-client",
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.hostnameOverride != nil {
				getHostname = c.hostnameOverride
				t.Cleanup(func() {
					getHostname = provideHostName
				})
			}

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
