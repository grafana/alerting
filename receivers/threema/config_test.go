package threema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

func TestNewConfig(t *testing.T) {
	var cases = []struct {
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
			expectedInitError: `could not find Threema Gateway ID in settings`,
		},
		{
			name: "Minimal valid configuration",
			settings: `{
				"gateway_id": "*1234567",
				"recipient_id": "12345678",
				"api_secret": "test-secret"
			}`,
			expectedConfig: Config{
				GatewayID:   "*1234567",
				RecipientID: "12345678",
				APISecret:   "test-secret",
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
			},
		},
		{
			name: "Minimal valid configuration from secrets",
			settings: `{
				"gateway_id": "*test123",
				"recipient_id": "test1234"
			}`,
			secureSettings: map[string][]byte{
				"api_secret": []byte("test-secret"),
			},
			expectedConfig: Config{
				GatewayID:   "*test123",
				RecipientID: "test1234",
				APISecret:   "test-secret",
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
			},
		},
		{
			name: "Error if gateway_id is missing",
			settings: `{
				"recipient_id": "*test123",
				"api_secret": "test-secret"
			}`,
			expectedInitError: `could not find Threema Gateway ID in settings`,
		},
		{
			name: "Error if recipient_id is missing",
			settings: `{
				"gateway_id": "*test123",
				"api_secret": "test-secret"
			}`,
			expectedInitError: `could not find Threema Recipient ID in settings`,
		},
		{
			name: "Error if api_secret is missing",
			settings: `{
				"gateway_id": "*1234567",
				"recipient_id": "test1234"
			}`,
			expectedInitError: `could not find Threema API secret in settings`,
		},
		{
			name: "Error if gateway does not start with *",
			settings: `{
				"gateway_id": "12345678",
				"recipient_id": "test1234",
				"api_secret": "test-secret"
			}`,
			expectedInitError: "invalid Threema Gateway ID: Must start with a *",
		},
		{
			name: "Error if gateway length is greater than 8",
			settings: `{
				"gateway_id": "*12345678",
				"recipient_id": "test1234",
				"api_secret": "test-secret"
			}`,
			expectedInitError: "invalid Threema Gateway ID: Must be 8 characters long",
		},
		{
			name: "Error if gateway length is less than 8",
			settings: `{
				"gateway_id": "*123456",
				"recipient_id": "*1234567",
				"api_secret": "test-secret"
			}`,
			expectedInitError: "invalid Threema Gateway ID: Must be 8 characters long",
		},
		{
			name: "Error if recipient_id length is greater than 8",
			settings: `{
				"gateway_id": "*1234567",
				"recipient_id": "123456789",
				"api_secret": "test-secret"
			}`,
			expectedInitError: "invalid Threema Recipient ID: Must be 8 characters long",
		},
		{
			name: "Error if recipient_id length is less than 8",
			settings: `{
				"gateway_id": "*1234567",
				"recipient_id": "123456",
				"api_secret": "test-secret"
			}`,
			expectedInitError: "invalid Threema Recipient ID: Must be 8 characters long",
		},
		{
			name: "All empty fields = minimal valid configuration",
			settings: `{
				"gateway_id": "*1234567",
				"recipient_id": "test-rec",
				"api_secret": "test-secret",
				"title" : "",
				"description": ""
			}`,
			expectedConfig: Config{
				GatewayID:   "*1234567",
				RecipientID: "test-rec",
				APISecret:   "test-secret",
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				GatewayID:   "*1234567",
				RecipientID: "*1234567",
				APISecret:   "test-secret",
				Title:       "test-title",
				Description: "test-description",
			},
		},
		{
			name:           "Extracts all fields + override from secrets",
			settings:       FullValidConfigForTesting,
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				GatewayID:   "*1234567",
				RecipientID: "*1234567",
				APISecret:   "test-secret-secret",
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
