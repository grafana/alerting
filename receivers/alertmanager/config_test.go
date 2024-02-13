package alertmanager

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
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
			name:              "Error if empty JSON object",
			settings:          `{}`,
			expectedInitError: `could not find url property in settings`,
		}, {
			name: "Error in initing: invalid URL",
			settings: `{
				"url": "://alertmanager.com"
			}`,
			expectedInitError: `invalid url property in settings: parse "://alertmanager.com/api/v2/alerts": missing protocol scheme`,
		},
		{
			name: "Error in initing: empty URL",
			settings: `{
				"url": ""
			}`,
			expectedInitError: `could not find url property in settings`,
		},
		{
			name: "Error in initing: null URL",
			settings: `{
				"url": null
			}`,
			expectedInitError: `could not find url property in settings`,
		},
		{
			name: "Error in initing: one of multiple URLs is invalid",
			settings: `{
				"url": "https://alertmanager-01.com,://url"
			}`,
			expectedInitError: "invalid url property in settings: parse \"://url/api/v2/alerts\": missing protocol scheme",
		}, {
			name: "Single URL",
			settings: `{
				"url": "https://alertmanager-01.com"
			}`,
			expectedConfig: Config{
				URLs: []*url.URL{
					receiversTesting.ParseURLUnsafe("https://alertmanager-01.com/api/v2/alerts"),
				},
				User:     "",
				Password: "",
			},
		},
		{
			name: "Comma-separated URLs",
			settings: `{
				"url": "https://alertmanager-01.com/,https://alertmanager-02.com,,https://alertmanager-03.com"
			}`,
			expectedConfig: Config{
				URLs: []*url.URL{
					receiversTesting.ParseURLUnsafe("https://alertmanager-01.com/api/v2/alerts"),
					receiversTesting.ParseURLUnsafe("https://alertmanager-02.com/api/v2/alerts"),
					receiversTesting.ParseURLUnsafe("https://alertmanager-03.com/api/v2/alerts"),
				},
				User:     "",
				Password: "",
			},
		},
		{
			name: "User and password plain",
			settings: `{
				"url": "https://alertmanager-01.com",
				"basicAuthUser": "grafana",
				"basicAuthPassword": "admin"
			}`,
			expectedConfig: Config{
				URLs: []*url.URL{
					receiversTesting.ParseURLUnsafe("https://alertmanager-01.com/api/v2/alerts"),
				},
				User:     "grafana",
				Password: "admin",
			},
		},
		{
			name:     "User and password from secrets",
			settings: FullValidConfigForTesting,
			secrets:  receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				URLs: []*url.URL{
					receiversTesting.ParseURLUnsafe("https://alertmanager-01.com/api/v2/alerts"),
				},
				User:     "grafana",
				Password: "grafana-admin",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sn, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(c.secrets))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}

			require.Equal(t, c.expectedConfig.User, sn.User)
			require.Equal(t, c.expectedConfig.Password, sn.Password)
			require.EqualValues(t, c.expectedConfig.URLs, sn.URLs)
		})
	}
}
