package webhook

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
	testing2 "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

func TestValidateConfig(t *testing.T) {
	cases := []struct {
		name              string
		settings          string
		secretSettings    map[string][]byte
		expectedConfig    Config
		expectedInitError string
	}{
		{
			name:              "Error if empty",
			settings:          "",
			expectedInitError: `failed to unmarshal settings: unexpected end of JSON input`,
		},
		{
			name:              "Error if empty JSON object",
			settings:          `{}`,
			expectedInitError: `required field 'url' is not specified`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"url": "http://localhost" }`,
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                0,
				AuthorizationScheme:      "",
				AuthorizationCredentials: "",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
		},
		{
			name: "All empty fields = minimal valid configuration",
			settings: `{
				"url": "http://localhost",
				"httpMethod": "",
				"maxAlerts": "",
				"authorization_scheme": "",
				"authorization_credentials": "",
				"username": "",
				"password": "",
				"title": "",
				"message": ""		
			}`,
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                0,
				AuthorizationScheme:      "",
				AuthorizationCredentials: "",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
		},
		{
			name: "Extracts all fields",
			settings: `{
				"url": "http://localhost",
				"httpMethod": "test-httpMethod",
				"maxAlerts": "2",
				"authorization_scheme": "basic",
				"authorization_credentials": "",
				"username": "test-user",
				"password": "test-pass",
				"title": "test-title",
				"message": "test-message"		
			}`,
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               "test-httpMethod",
				MaxAlerts:                2,
				AuthorizationScheme:      "basic",
				AuthorizationCredentials: "",
				User:                     "test-user",
				Password:                 "test-pass",
				Title:                    "test-title",
				Message:                  "test-message",
			},
		},
		{
			name:     "should parse maxAlerts as string",
			settings: `{"url": "http://localhost", "maxAlerts": "12" }`,
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                12,
				AuthorizationScheme:      "",
				AuthorizationCredentials: "",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "should parse maxAlerts as number",
			settings: `{"url": "http://localhost", "maxAlerts": 12 }`,
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                12,
				AuthorizationScheme:      "",
				AuthorizationCredentials: "",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "should default to 0 if maxAlerts is not valid number",
			settings: `{"url": "http://localhost", "maxAlerts": "test-max-alerts" }`,
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                0,
				AuthorizationScheme:      "",
				AuthorizationCredentials: "",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "should extract username and password from secrets",
			settings: `{"url": "http://localhost" }`,
			secretSettings: map[string][]byte{
				"username": []byte("test-user"),
				"password": []byte("test-password"),
			},
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                0,
				AuthorizationScheme:      "",
				AuthorizationCredentials: "",
				User:                     "test-user",
				Password:                 "test-password",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "should override username and password from secrets",
			settings: `{"url": "http://localhost", "username": "test", "password" : "test" }`,
			secretSettings: map[string][]byte{
				"username": []byte("test-user"),
				"password": []byte("test-password"),
			},
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                0,
				AuthorizationScheme:      "",
				AuthorizationCredentials: "",
				User:                     "test-user",
				Password:                 "test-password",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "should extract authorization_credentials from secrets",
			settings: `{"url": "http://localhost", "authorization_scheme" : "test-scheme" }`,
			secretSettings: map[string][]byte{
				"authorization_credentials": []byte("test-authorization_credentials"),
			},
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                0,
				AuthorizationScheme:      "test-scheme",
				AuthorizationCredentials: "test-authorization_credentials",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "should override authorization_credentials from secrets",
			settings: `{"url": "http://localhost", "authorization_scheme" : "test-scheme",  "authorization_credentials": "test" }`,
			secretSettings: map[string][]byte{
				"authorization_credentials": []byte("test-authorization_credentials"),
			},
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                0,
				AuthorizationScheme:      "test-scheme",
				AuthorizationCredentials: "test-authorization_credentials",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
		},
		{
			name:     "should default authorization_scheme to Bearer if authorization_credentials specified",
			settings: `{"url": "http://localhost" }`,
			secretSettings: map[string][]byte{
				"authorization_credentials": []byte("test-authorization_credentials"),
			},
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                0,
				AuthorizationScheme:      "Bearer",
				AuthorizationCredentials: "test-authorization_credentials",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
		},
		{
			name: "error if both credentials are specified",
			settings: `{
				"url": "http://localhost",
				"authorization_scheme": "basic",
				"authorization_credentials": "test-credentials",
				"username": "test-user",
				"password": "test-pass"
			}`,
			expectedInitError: "both HTTP Basic Authentication and Authorization Header are set, only 1 is permitted",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &receivers.NotificationChannelConfig{
				Settings:       json.RawMessage(c.settings),
				SecureSettings: c.secretSettings,
			}
			fc, err := testing2.NewFactoryConfigForValidateConfigTesting(t, m)
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
