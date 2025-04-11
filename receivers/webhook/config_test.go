package webhook

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

func TestNewConfig(t *testing.T) {
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
			expectedInitError: `failed to unmarshal settings`,
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
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
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
				TLSConfig: &receivers.TLSConfig{
					InsecureSkipVerify: false,
					ClientCertificate:  "test-client-certificate",
					ClientKey:          "test-client-key",
					CACertificate:      "test-ca-certificate",
				},
				HMACConfig: &receivers.HMACConfig{
					Secret:          "test-hmac-secret",
					Header:          "X-Grafana-Alerting-Signature",
					TimestampHeader: "X-Grafana-Alerting-Timestamp",
				},
			},
		},
		{
			name:           "Extracts all fields + override from secrets",
			settings:       FullValidConfigForTesting,
			secretSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				URL:                      "http://localhost",
				HTTPMethod:               "test-httpMethod",
				MaxAlerts:                2,
				AuthorizationScheme:      "basic",
				AuthorizationCredentials: "",
				User:                     "test-secret-user",
				Password:                 "test-secret-pass",
				Title:                    "test-title",
				Message:                  "test-message",
				TLSConfig: &receivers.TLSConfig{
					InsecureSkipVerify: false,
					ClientCertificate:  "test-override-client-certificate",
					ClientKey:          "test-override-client-key",
					CACertificate:      "test-override-ca-certificate",
				},
				HMACConfig: &receivers.HMACConfig{
					Secret:          "test-override-hmac-secret",
					Header:          "X-Grafana-Alerting-Signature",
					TimestampHeader: "X-Grafana-Alerting-Timestamp",
				},
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
			name: "with HMAC config",
			settings: `{
				"url": "http://localhost/test1",
				"hmacConfig": {
					"header": "X-Test-Hash",
					"timestampHeader": "X-Test-Timestamp"
				}
			}`,
			secretSettings: map[string][]byte{
				"hmacConfig.secret": []byte("test-secret-from-secrets"),
			},
			expectedConfig: Config{
				URL:        "http://localhost/test1",
				HTTPMethod: http.MethodPost,
				MaxAlerts:  0,
				Title:      templates.DefaultMessageTitleEmbed,
				Message:    templates.DefaultMessageEmbed,
				HMACConfig: &receivers.HMACConfig{
					Secret:          "test-secret-from-secrets",
					Header:          "X-Test-Hash",
					TimestampHeader: "X-Test-Timestamp",
				},
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
		{
			name: "with custom payload and variables",
			settings: `{
				"url": "http://localhost/test1",
				"payload": {
					"template": "{{ define \"test\" }}{{ .Receiver }}{{ .Vars.var1 }}{{ end }}",
					"vars": {
						"var1": "variablevalue"
					}
				}
			}`,
			expectedConfig: Config{
				URL:        "http://localhost/test1",
				HTTPMethod: http.MethodPost,
				MaxAlerts:  0,
				Title:      templates.DefaultMessageTitleEmbed,
				Message:    templates.DefaultMessageEmbed,
				Payload: CustomPayload{
					Template: `{{ define "test" }}{{ .Receiver }}{{ .Vars.var1 }}{{ end }}`,
					Vars: map[string]string{
						"var1": "variablevalue",
					},
				},
			},
		},
		{
			name: "with custom headers",
			settings: `{
				"url": "http://localhost/test1",
				"headers": {
					"X-Test-Header": "test-header-value",
					"Content-Type": "test-content-type"
				}
			}`,
			expectedConfig: Config{
				URL:        "http://localhost/test1",
				HTTPMethod: http.MethodPost,
				MaxAlerts:  0,
				Title:      templates.DefaultMessageTitleEmbed,
				Message:    templates.DefaultMessageEmbed,
				ExtraHeaders: map[string]string{
					"X-Test-Header": "test-header-value",
					"Content-Type":  "test-content-type",
				},
			},
		},
		{
			name: "with restricted custom headers",
			settings: func() string {
				headers := map[string]string{
					"X-Test-Header": "test-header-value",
					"Content-Type":  "test-content-type",
				}
				for k := range restrictedHeaders {
					headers[strings.ToLower(k)] = k // Also make sure it handled non-canonical headers.
				}
				data, _ := json.Marshal(struct {
					URL     string            `json:"url"`
					Headers map[string]string `json:"headers"`
				}{
					URL:     "http://localhost/test1",
					Headers: headers,
				})
				return string(data)
			}(),
			expectedInitError: "custom headers [\"accept-charset\" \"accept-encoding\" \"authorization\" \"connection\" \"content-encoding\" \"content-length\" \"cookie\" \"date\" \"expect\" \"forwarded\" \"host\" \"keep-alive\" \"max-forwards\" \"origin\" \"proxy-authenticate\" \"proxy-authorization\" \"referer\" \"set-cookie\" \"te\" \"trailer\" \"transfer-encoding\" \"upgrade\" \"user-agent\" \"via\" \"x-forwarded-for\" \"x-real-ip\"] are not allowed",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(c.secretSettings))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
