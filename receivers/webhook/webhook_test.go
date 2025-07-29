package webhook

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

//go:embed fixtures/ca.pem
var caCert string

//go:embed fixtures/client.pem
var clientCert string

//go:embed fixtures/client.key
var clientKey string

func TestNotify(t *testing.T) {
	tmpl := templates.ForTests(t)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	orgID := int64(1)

	cases := []struct {
		name     string
		settings Config
		alerts   []*types.Alert

		expMsg        *webhookMessage
		expURL        string
		expUsername   string
		expPassword   string
		expHeaders    map[string]string
		expHTTPMethod string
		expMsgError   error
	}{
		{
			name: "Default config with one alert with custom message",
			settings: Config{
				URL:                      "http://localhost/test",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                0,
				AuthorizationScheme:      "",
				AuthorizationCredentials: "",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  "Custom message",
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expURL:        "http://localhost/test",
			expHTTPMethod: "POST",
			expMsg: &webhookMessage{
				ExtendedData: &templates.ExtendedData{
					Receiver: "my_receiver",
					Status:   "firing",
					Alerts: templates.ExtendedAlerts{
						{
							Status: "firing",
							Labels: templates.KV{
								"alertname": "alert1",
								"lbl1":      "val1",
							},
							Annotations: templates.KV{
								"ann1": "annv1",
							},
							Fingerprint:  "fac0861a85de433a",
							DashboardURL: "http://localhost/d/abcd",
							PanelURL:     "http://localhost/d/abcd?viewPanel=efgh",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1",
						},
					},
					GroupLabels: templates.KV{
						"alertname": "",
					},
					CommonLabels: templates.KV{
						"alertname": "alert1",
						"lbl1":      "val1",
					},
					CommonAnnotations: templates.KV{
						"ann1": "annv1",
					},
					ExternalURL: "http://localhost",
				},
				Version:  "1",
				GroupKey: "alertname",
				Title:    "[FIRING:1]  (val1)",
				State:    "alerting",
				Message:  "Custom message",
				OrgID:    orgID,
			},
			expMsgError: nil,
			expHeaders:  map[string]string{},
		},
		{
			name: "Custom config with multiple alerts with custom title",
			settings: Config{
				URL:                      "http://localhost/test1",
				HTTPMethod:               "PUT",
				MaxAlerts:                2,
				AuthorizationScheme:      "",
				AuthorizationCredentials: "",
				User:                     "user1",
				Password:                 "mysecret",
				Title:                    "Alerts firing: {{ len .Alerts.Firing }}",
				Message:                  templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val3"},
						Annotations: model.LabelSet{"ann1": "annv3"},
					},
				},
			},
			expURL:        "http://localhost/test1",
			expHTTPMethod: "PUT",
			expUsername:   "user1",
			expPassword:   "mysecret",
			expMsg: &webhookMessage{
				ExtendedData: &templates.ExtendedData{
					Receiver: "my_receiver",
					Status:   "firing",
					Alerts: templates.ExtendedAlerts{
						{
							Status: "firing",
							Labels: templates.KV{
								"alertname": "alert1",
								"lbl1":      "val1",
							},
							Annotations: templates.KV{
								"ann1": "annv1",
							},
							Fingerprint: "fac0861a85de433a",
							SilenceURL:  "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1",
						}, {
							Status: "firing",
							Labels: templates.KV{
								"alertname": "alert1",
								"lbl1":      "val2",
							},
							Annotations: templates.KV{
								"ann1": "annv2",
							},
							Fingerprint: "fab6861a85d5eeb5",
							SilenceURL:  "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2",
						},
					},
					GroupLabels: templates.KV{
						"alertname": "",
					},
					CommonLabels: templates.KV{
						"alertname": "alert1",
					},
					CommonAnnotations: templates.KV{},
					ExternalURL:       "http://localhost",
				},
				Version:         "1",
				GroupKey:        "alertname",
				TruncatedAlerts: 1,
				Title:           "Alerts firing: 2",
				State:           "alerting",
				Message:         "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
				OrgID:           orgID,
			},
			expMsgError: nil,
			expHeaders:  map[string]string{},
		},
		{
			name: "Default config, template variables in URL",
			settings: Config{
				URL:                      "http://localhost/test?numAlerts={{len .Alerts}}&status={{.Status}}",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                0,
				AuthorizationScheme:      "",
				AuthorizationCredentials: "",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2"},
					},
				},
			},
			expURL:        "http://localhost/test?numAlerts=2&status=firing",
			expHTTPMethod: "POST",
			expMsg: &webhookMessage{
				ExtendedData: &templates.ExtendedData{
					Receiver: "my_receiver",
					Status:   "firing",
					Alerts: templates.ExtendedAlerts{
						{
							Status: "firing",
							Labels: templates.KV{
								"alertname": "alert1",
								"lbl1":      "val1",
							},
							Annotations: templates.KV{
								"ann1": "annv1",
							},
							Fingerprint: "fac0861a85de433a",
							SilenceURL:  "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1",
						}, {
							Status: "firing",
							Labels: templates.KV{
								"alertname": "alert1",
								"lbl1":      "val2",
							},
							Annotations: templates.KV{
								"ann1": "annv2",
							},
							Fingerprint: "fab6861a85d5eeb5",
							SilenceURL:  "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2",
						},
					},
					GroupLabels: templates.KV{
						"alertname": "",
					},
					CommonLabels: templates.KV{
						"alertname": "alert1",
					},
					CommonAnnotations: templates.KV{},
					ExternalURL:       "http://localhost",
				},
				Version:         "1",
				GroupKey:        "alertname",
				TruncatedAlerts: 0,
				Title:           "[FIRING:2]  ",
				State:           "alerting",
				Message:         "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
				OrgID:           orgID,
			},
			expMsgError: nil,
			expHeaders:  map[string]string{},
		},
		{
			name: "with Authorization set",
			settings: Config{
				URL:                      "http://localhost/test1",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                2,
				AuthorizationScheme:      "Bearer",
				AuthorizationCredentials: "mysecret",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &webhookMessage{
				ExtendedData: &templates.ExtendedData{
					Receiver: "my_receiver",
					Status:   "firing",
					Alerts: templates.ExtendedAlerts{
						{
							Status: "firing",
							Labels: templates.KV{
								"alertname": "alert1",
								"lbl1":      "val1",
							},
							Annotations: templates.KV{
								"ann1": "annv1",
							},
							Fingerprint:  "fac0861a85de433a",
							DashboardURL: "http://localhost/d/abcd",
							PanelURL:     "http://localhost/d/abcd?viewPanel=efgh",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1",
						},
					},
					GroupLabels: templates.KV{
						"alertname": "",
					},
					CommonLabels: templates.KV{
						"alertname": "alert1",
						"lbl1":      "val1",
					},
					CommonAnnotations: templates.KV{
						"ann1": "annv1",
					},
					ExternalURL: "http://localhost",
				},
				Version:  "1",
				GroupKey: "alertname",
				Title:    "[FIRING:1]  (val1)",
				State:    "alerting",
				Message:  "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				OrgID:    orgID,
			},
			expURL:        "http://localhost/test1",
			expHTTPMethod: "POST",
			expHeaders:    map[string]string{"Authorization": "Bearer mysecret"},
		},
		{
			name: "with TLSConfig set and insecure skip verify",
			settings: Config{
				URL:                      "https://localhost/test1",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                2,
				AuthorizationScheme:      "Bearer",
				AuthorizationCredentials: "mysecret",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
				TLSConfig: &receivers.TLSConfig{
					CACertificate:      caCert,
					ClientKey:          clientKey,
					ClientCertificate:  clientCert,
					InsecureSkipVerify: true,
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &webhookMessage{
				ExtendedData: &templates.ExtendedData{
					Receiver: "my_receiver",
					Status:   "firing",
					Alerts: templates.ExtendedAlerts{
						{
							Status: "firing",
							Labels: templates.KV{
								"alertname": "alert1",
								"lbl1":      "val1",
							},
							Annotations: templates.KV{
								"ann1": "annv1",
							},
							Fingerprint:  "fac0861a85de433a",
							DashboardURL: "http://localhost/d/abcd",
							PanelURL:     "http://localhost/d/abcd?viewPanel=efgh",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1",
						},
					},
					GroupLabels: templates.KV{
						"alertname": "",
					},
					CommonLabels: templates.KV{
						"alertname": "alert1",
						"lbl1":      "val1",
					},
					CommonAnnotations: templates.KV{
						"ann1": "annv1",
					},
					ExternalURL: "http://localhost",
				},
				Version:  "1",
				GroupKey: "alertname",
				Title:    "[FIRING:1]  (val1)",
				State:    "alerting",
				Message:  "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				OrgID:    orgID,
			},
			expURL:        "https://localhost/test1",
			expHTTPMethod: "POST",
			expHeaders:    map[string]string{"Authorization": "Bearer mysecret"},
		},
		{
			name: "with TLSConfig set",
			settings: Config{
				URL:                      "https://localhost/test1",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                2,
				AuthorizationScheme:      "Bearer",
				AuthorizationCredentials: "mysecret",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
				TLSConfig: &receivers.TLSConfig{
					CACertificate:      caCert,
					ClientKey:          clientKey,
					ClientCertificate:  clientCert,
					InsecureSkipVerify: false,
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &webhookMessage{
				ExtendedData: &templates.ExtendedData{
					Receiver: "my_receiver",
					Status:   "firing",
					Alerts: templates.ExtendedAlerts{
						{
							Status: "firing",
							Labels: templates.KV{
								"alertname": "alert1",
								"lbl1":      "val1",
							},
							Annotations: templates.KV{
								"ann1": "annv1",
							},
							Fingerprint:  "fac0861a85de433a",
							DashboardURL: "http://localhost/d/abcd",
							PanelURL:     "http://localhost/d/abcd?viewPanel=efgh",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1",
						},
					},
					GroupLabels: templates.KV{
						"alertname": "",
					},
					CommonLabels: templates.KV{
						"alertname": "alert1",
						"lbl1":      "val1",
					},
					CommonAnnotations: templates.KV{
						"ann1": "annv1",
					},
					ExternalURL: "http://localhost",
				},
				Version:  "1",
				GroupKey: "alertname",
				Title:    "[FIRING:1]  (val1)",
				State:    "alerting",
				Message:  "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				OrgID:    orgID,
			},
			expURL:        "https://localhost/test1",
			expHTTPMethod: "POST",
			expHeaders:    map[string]string{"Authorization": "Bearer mysecret"},
		},
		{
			name: "with HMAC config",
			settings: Config{
				URL:        "http://localhost/test1",
				HTTPMethod: http.MethodPost,
				MaxAlerts:  2,
				Title:      templates.DefaultMessageTitleEmbed,
				Message:    templates.DefaultMessageEmbed,
				HMACConfig: &receivers.HMACConfig{
					Secret:          "test-secret",
					Header:          "X-Test-Hash",
					TimestampHeader: "X-Test-Timestamp",
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
			expMsg: &webhookMessage{
				ExtendedData: &templates.ExtendedData{
					Receiver: "my_receiver",
					Status:   "firing",
					Alerts: templates.ExtendedAlerts{
						{
							Status: "firing",
							Labels: templates.KV{
								"alertname": "alert1",
								"lbl1":      "val1",
							},
							Annotations: templates.KV{
								"ann1": "annv1",
							},
							Fingerprint: "fac0861a85de433a",
							SilenceURL:  "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1",
						},
					},
					GroupLabels: templates.KV{
						"alertname": "",
					},
					CommonLabels: templates.KV{
						"alertname": "alert1",
						"lbl1":      "val1",
					},
					CommonAnnotations: templates.KV{
						"ann1": "annv1",
					},
					ExternalURL: "http://localhost",
				},
				Version:  "1",
				GroupKey: "alertname",
				Title:    "[FIRING:1]  (val1)",
				State:    "alerting",
				Message:  "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n",
				OrgID:    orgID,
			},
			expURL:        "http://localhost/test1",
			expHTTPMethod: "POST",
			expHeaders:    map[string]string{},
		},
		{
			name: "bad CA certificate set",
			settings: Config{
				URL:                      "http://localhost/test1",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                2,
				AuthorizationScheme:      "Bearer",
				AuthorizationCredentials: "mysecret",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
				TLSConfig: &receivers.TLSConfig{
					CACertificate:      "fake_ca",
					ClientKey:          "fake_key",
					ClientCertificate:  "fake_cert",
					InsecureSkipVerify: false,
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsgError: fmt.Errorf("Unable to use the provided CA certificate"),
		},
		{
			name: "bad template in url",
			settings: Config{
				URL:                      "http://localhost/test1?numAlerts={{len Alerts}}",
				HTTPMethod:               http.MethodPost,
				MaxAlerts:                0,
				AuthorizationScheme:      "",
				AuthorizationCredentials: "",
				User:                     "",
				Password:                 "",
				Title:                    templates.DefaultMessageTitleEmbed,
				Message:                  templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsgError: fmt.Errorf("template: :1: function \"Alerts\" not defined"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			webhookSender := receivers.MockNotificationService()

			pn := &Notifier{
				Base: &receivers.Base{
					Name:                  "",
					Type:                  "",
					UID:                   "",
					DisableResolveMessage: false,
				},
				log:      &logging.FakeLogger{},
				ns:       webhookSender,
				tmpl:     tmpl,
				settings: c.settings,
				images:   &images.UnavailableProvider{},
				orgID:    1,
			}

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
			ctx = notify.WithReceiverName(ctx, "my_receiver")
			ok, err := pn.Notify(ctx, c.alerts...)
			if c.expMsgError != nil {
				require.False(t, ok)
				require.Error(t, err)
				require.Equal(t, c.expMsgError.Error(), err.Error())
				return
			}
			require.NoError(t, err)
			require.True(t, ok)

			expBody, err := json.Marshal(c.expMsg)
			require.NoError(t, err)

			require.JSONEq(t, string(expBody), webhookSender.Webhook.Body)
			require.Equal(t, c.expURL, webhookSender.Webhook.URL)
			require.Equal(t, c.expUsername, webhookSender.Webhook.User)
			require.Equal(t, c.expPassword, webhookSender.Webhook.Password)
			require.Equal(t, c.expHTTPMethod, webhookSender.Webhook.HTTPMethod)
			require.Equal(t, c.expHeaders, webhookSender.Webhook.HTTPHeader)
		})
	}
}
