package alertmanager

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	testing2 "github.com/grafana/alerting/receivers/testing"
)

func TestValidateConfig(t *testing.T) {
	cases := []struct {
		name              string
		settings          string
		secrets           map[string][]byte
		expectedConfig    Config
		expectedInitError string
	}{
		{
			name:              "Error in initing: missing URL",
			settings:          `{}`,
			expectedInitError: `could not find url property in settings`,
		}, {
			name: "Error in initing: invalid URL",
			settings: `{
				"url": "://alertmanager.com"
			}`,
			expectedInitError: `invalid url property in settings: parse "://alertmanager.com/api/v1/alerts": missing protocol scheme`,
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
			expectedInitError: "invalid url property in settings: parse \"://url/api/v1/alerts\": missing protocol scheme",
		}, {
			name: "Single URL",
			settings: `{
				"url": "https://alertmanager-01.com"
			}`,
			expectedConfig: Config{
				URLs: []*url.URL{
					testing2.ParseURLUnsafe("https://alertmanager-01.com/api/v1/alerts"),
				},
				User:     "",
				Password: "",
			},
		},
		{
			name: "Comma-separated URLs",
			settings: `{
				"url": "https://alertmanager-01.com/,https://alertmanager-02.com, https://alertmanager-03.com"
			}`,
			expectedConfig: Config{
				URLs: []*url.URL{
					testing2.ParseURLUnsafe("https://alertmanager-01.com/api/v1/alerts"),
					testing2.ParseURLUnsafe("https://alertmanager-02.com/api/v1/alerts"),
					testing2.ParseURLUnsafe("https://alertmanager-03.com/api/v1/alerts"),
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
					testing2.ParseURLUnsafe("https://alertmanager-01.com/api/v1/alerts"),
				},
				User:     "grafana",
				Password: "admin",
			},
		},
		{
			name: "User and password from secrets",
			settings: `{
				"url": "https://alertmanager-01.com",
				"basicAuthUser": "grafana",
				"basicAuthPassword": "admin"
			}`,
			secrets: map[string][]byte{
				"basicAuthPassword": []byte("grafana-admin"),
			},
			expectedConfig: Config{
				URLs: []*url.URL{
					testing2.ParseURLUnsafe("https://alertmanager-01.com/api/v1/alerts"),
				},
				User:     "grafana",
				Password: "grafana-admin",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &receivers.NotificationChannelConfig{
				Name:           "Alertmanager",
				Type:           Type,
				Settings:       json.RawMessage(c.settings),
				SecureSettings: c.secrets,
			}
			fc, err := testing2.NewFactoryConfigForValidateConfigTesting(t, m)
			require.NoError(t, err)

			sn, err := ValidateConfig(fc)

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

func TestAlertmanagerNotifier_Notify(t *testing.T) {
	imageStore := images.NewFakeImageStore(1)
	singleURLConfig := Config{
		URLs: []*url.URL{
			testing2.ParseURLUnsafe("https://alertmanager.com/api/v1/alerts"),
		},
		User:     "admin",
		Password: "password",
	}

	cases := []struct {
		name                 string
		settings             Config
		alerts               []*types.Alert
		expectedError        string
		sendHTTPRequestError error
	}{
		{
			name:     "Default config with one alert",
			settings: singleURLConfig,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
		}, {
			name:     "Default config with one alert with image URL",
			settings: singleURLConfig,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1"},
						Annotations: model.LabelSet{"__alertImageToken__": "test-image-1"},
					},
				},
			},
		}, {
			name:     "Default config with one alert with empty receiver name",
			settings: singleURLConfig,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
		}, {
			name:     "Error sending to Alertmanager",
			settings: singleURLConfig,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
			expectedError:        "failed to send alert to Alertmanager: expected error",
			sendHTTPRequestError: errors.New("expected error"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sn := &Notifier{
				Base: &receivers.Base{
					Name:                  "",
					Type:                  "",
					UID:                   "",
					DisableResolveMessage: false,
				},
				images:   imageStore,
				settings: c.settings,
				logger:   &logging.FakeLogger{},
			}

			var body []byte
			origSendHTTPRequest := receivers.SendHTTPRequest
			t.Cleanup(func() {
				receivers.SendHTTPRequest = origSendHTTPRequest
			})
			receivers.SendHTTPRequest = func(ctx context.Context, url *url.URL, cfg receivers.HTTPCfg, logger logging.Logger) ([]byte, error) {
				body = cfg.Body
				assert.Equal(t, c.settings.User, cfg.User)
				assert.Equal(t, string(c.settings.Password), cfg.Password)
				return nil, c.sendHTTPRequestError
			}

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
			ok, err := sn.Notify(ctx, c.alerts...)

			if c.sendHTTPRequestError != nil {
				require.EqualError(t, err, c.expectedError)
				require.False(t, ok)
			} else {
				require.NoError(t, err)
				require.True(t, ok)
				expBody, err := json.Marshal(c.alerts)
				require.NoError(t, err)
				require.JSONEq(t, string(expBody), string(body))
			}
		})
	}
}
