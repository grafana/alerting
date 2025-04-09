package webhook

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/models"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

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

// DefaultPayloadTemplate is meant to mimic the default payload used by webhook receivers when no custom payload template is provided.
const DefaultPayloadTemplate = `{{ template "webhook.default.payload" . }}`

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
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
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
							DashboardURL: "http://localhost/d/abcd?orgId=1",
							PanelURL:     "http://localhost/d/abcd?orgId=1&viewPanel=efgh",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1",
							GeneratorURL: "http://localhost/test?orgId=1",
							OrgID:        &orgID,
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
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
					},
				}, {
					Alert: model.Alert{
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations:  model.LabelSet{"ann1": "annv2", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
					},
				}, {
					Alert: model.Alert{
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val3"},
						Annotations:  model.LabelSet{"ann1": "annv3", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
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
							Fingerprint:  "fac0861a85de433a",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1",
							GeneratorURL: "http://localhost/test?orgId=1",
							OrgID:        &orgID,
						}, {
							Status: "firing",
							Labels: templates.KV{
								"alertname": "alert1",
								"lbl1":      "val2",
							},
							Annotations: templates.KV{
								"ann1": "annv2",
							},
							Fingerprint:  "fab6861a85d5eeb5",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2&orgId=1",
							GeneratorURL: "http://localhost/test?orgId=1",
							OrgID:        &orgID,
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
				Message:         "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/test?orgId=1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSource: http://localhost/test?orgId=1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2&orgId=1\n",
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
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
					},
				}, {
					Alert: model.Alert{
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations:  model.LabelSet{"ann1": "annv2", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
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
							Fingerprint:  "fac0861a85de433a",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1",
							GeneratorURL: "http://localhost/test?orgId=1",
							OrgID:        &orgID,
						}, {
							Status: "firing",
							Labels: templates.KV{
								"alertname": "alert1",
								"lbl1":      "val2",
							},
							Annotations: templates.KV{
								"ann1": "annv2",
							},
							Fingerprint:  "fab6861a85d5eeb5",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2&orgId=1",
							GeneratorURL: "http://localhost/test?orgId=1",
							OrgID:        &orgID,
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
				Message:         "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/test?orgId=1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSource: http://localhost/test?orgId=1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2&orgId=1\n",
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
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
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
							DashboardURL: "http://localhost/d/abcd?orgId=1",
							PanelURL:     "http://localhost/d/abcd?orgId=1&viewPanel=efgh",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1",
							GeneratorURL: "http://localhost/test?orgId=1",
							OrgID:        &orgID,
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
				Message:  "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/test?orgId=1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1\nDashboard: http://localhost/d/abcd?orgId=1\nPanel: http://localhost/d/abcd?orgId=1&viewPanel=efgh\n",
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
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
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
							DashboardURL: "http://localhost/d/abcd?orgId=1",
							PanelURL:     "http://localhost/d/abcd?orgId=1&viewPanel=efgh",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1",
							GeneratorURL: "http://localhost/test?orgId=1",
							OrgID:        &orgID,
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
				Message:  "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/test?orgId=1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1\nDashboard: http://localhost/d/abcd?orgId=1\nPanel: http://localhost/d/abcd?orgId=1&viewPanel=efgh\n",
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
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
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
							DashboardURL: "http://localhost/d/abcd?orgId=1",
							PanelURL:     "http://localhost/d/abcd?orgId=1&viewPanel=efgh",
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1",
							GeneratorURL: "http://localhost/test?orgId=1",
							OrgID:        &orgID,
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
				Message:  "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/test?orgId=1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1\nDashboard: http://localhost/d/abcd?orgId=1\nPanel: http://localhost/d/abcd?orgId=1&viewPanel=efgh\n",
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
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
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
							SilenceURL:   "http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1",
							GeneratorURL: "http://localhost/test?orgId=1",
							OrgID:        &orgID,
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
				Message:  "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/test?orgId=1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1\n",
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
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
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
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
					},
				},
			},
			expMsgError: fmt.Errorf("template: :1: function \"Alerts\" not defined"),
		},
		{
			name: "with extra headers",
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
				ExtraHeaders: map[string]string{
					"X-Test-Header":    "TestValue",
					"X-Another-Header": "AnotherValue",
					"Content-Type":     "application/text",
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
					},
				},
			},
			expURL:        "https://localhost/test1",
			expHTTPMethod: "POST",
			expHeaders: map[string]string{
				"Authorization":    "Bearer mysecret",
				"X-Test-Header":    "TestValue",
				"X-Another-Header": "AnotherValue",
				"Content-Type":     "application/text",
			},
		},
		{
			name: "with restricted headers",
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
				ExtraHeaders: func() map[string]string {
					headers := map[string]string{
						"X-Test-Header":    "TestValue",
						"X-Another-Header": "AnotherValue",
						"Content-Type":     "application/text",
					}
					for k := range restrictedHeaders {
						headers[strings.ToLower(k)] = k // Also make sure it handled non-canonical headers.
					}
					return headers
				}(),
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
					},
				},
			},
			expURL:        "https://localhost/test1",
			expHTTPMethod: "POST",
			expHeaders: map[string]string{
				"Authorization":    "Bearer mysecret",
				"X-Test-Header":    "TestValue",
				"X-Another-Header": "AnotherValue",
				"Content-Type":     "application/text",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// We test both the default payload and the default templated payload. This helps ensure that payload
			// templating works as expected and also that the default payload is consistent with the templated payload.
			for _, payloadTemplate := range []string{"", DefaultPayloadTemplate} {
				testName := "default payload"
				settings := c.settings
				if payloadTemplate != "" {
					testName = "default templated payload"
					settings.Payload.Template = payloadTemplate

					if c.settings.Message != templates.DefaultMessageEmbed || c.settings.Title != templates.DefaultMessageTitleEmbed {
						// Not good candidates for testing the templated payload as it doesn't use the Message and Title fields.
						continue
					}
				}
				t.Run(testName, func(t *testing.T) {
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
						settings: settings,
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

					if c.expMsg != nil {
						expBody, err := json.Marshal(c.expMsg)
						require.NoError(t, err)
						require.JSONEq(t, string(expBody), webhookSender.Webhook.Body)
					}
					require.Equal(t, c.expURL, webhookSender.Webhook.URL)
					require.Equal(t, c.expUsername, webhookSender.Webhook.User)
					require.Equal(t, c.expPassword, webhookSender.Webhook.Password)
					require.Equal(t, c.expHTTPMethod, webhookSender.Webhook.HTTPMethod)
					require.Equal(t, c.expHeaders, webhookSender.Webhook.HTTPHeader)
				})
			}
		})
	}
}

func TestNotify_CustomPayload(t *testing.T) {
	tmpl := templates.ForTests(t)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	orgID := int64(1)

	cases := []struct {
		name     string
		settings Config
		alerts   []*types.Alert

		expPlaintext bool
		expMsg       string
		expMsgError  error
	}{
		{
			name: "Custom payload with one alert",
			settings: Config{
				URL:        "http://localhost/test",
				HTTPMethod: http.MethodPost,
				Payload: CustomPayload{
					Template: `{{ template "webhook.default.payload" . }}`,
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						GeneratorURL: "http://localhost/test",
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
					},
				},
			},
			expMsg: `{
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "alert1",
        "lbl1": "val1"
      },
      "annotations": {
        "ann1": "annv1"
      },
      "startsAt": "0001-01-01T00:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://localhost/test?orgId=1",
      "fingerprint": "fac0861a85de433a",
      "silenceURL": "http://localhost/alerting/silence/new?alertmanager=grafana\u0026matcher=alertname%3Dalert1\u0026matcher=lbl1%3Dval1&orgId=1",
      "dashboardURL": "",
      "orgId": 1,
      "panelURL": "",
      "values": null,
      "valueString": ""
    }
  ],
  "commonAnnotations": {
    "ann1": "annv1"
  },
  "commonLabels": {
    "alertname": "alert1",
    "lbl1": "val1"
  },
  "externalURL": "http://localhost",
  "groupKey": "alertname",
  "groupLabels": {
    "alertname": ""
  },
  "message": "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/test?orgId=1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana\u0026matcher=alertname%3Dalert1\u0026matcher=lbl1%3Dval1&orgId=1\n",
  "orgId": 1,
  "receiver": "my_receiver",
  "state": "alerting",
  "status": "firing",
  "title": "[FIRING:1]  (val1)",
  "truncatedAlerts": 0,
  "version": "1"
}`,
			expMsgError: nil,
		},

		{
			name: "variables and extra fields",
			settings: Config{
				URL:        "http://localhost/test",
				HTTPMethod: http.MethodPost,
				MaxAlerts:  1,
				Payload: CustomPayload{
					Template: `{{ .Vars | data.ToJSONPretty " " }}`,
					Vars: map[string]string{
						"var1": "val1",
						"var2": "val2",
					},
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
						GeneratorURL: "http://localhost/generator",
					},
				},
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert2", "lbl1": "val2"},
						Annotations:  model.LabelSet{"ann2": "annv2", models.OrgIDAnnotation: model.LabelValue(fmt.Sprint(orgID))},
						GeneratorURL: "http://localhost/generator",
					},
				},
			},
			expMsg: `
{
  "var1": "val1",
  "var2": "val2"
 }
`,
			expMsgError: nil,
		},
		{
			name: "Alertmanager-like payload",
			settings: Config{
				URL:        "http://localhost/test",
				HTTPMethod: http.MethodPost,
				Payload: CustomPayload{
					Template: `{{- $alerts := coll.Slice -}}
  {{- range .Alerts -}}
    {{- $alerts = coll.Append (coll.Dict 
    "labels" .Labels
    "annotations" .Annotations
    "startsAt" .StartsAt
    "endsAt" .EndsAt
    "generatorURL" .GeneratorURL
    ) $alerts }}
  {{- end }}
  {{- $alerts | data.ToJSONPretty " " }}`,
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1"},
						StartsAt:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
						GeneratorURL: "http://localhost/generator",
					},
				},
			},
			expMsg: `
[
 {
  "startsAt": "2025-01-01T00:00:00Z",
  "endsAt": "0001-01-01T00:00:00Z",
  "generatorURL": "http://localhost/generator",
  "labels": {
   "alertname": "alert1",
   "lbl1": "val1"
  },
  "annotations": {
   "ann1": "annv1"
  }
 }
]
`,
			expMsgError: nil,
		},
		{
			name: "dingding-like payload",
			settings: Config{
				URL:        "http://localhost/test",
				HTTPMethod: http.MethodPost,
				Payload: CustomPayload{
					Template: `{{ $url := print .ExternalURL  "/alerting/list" -}}
  {{ coll.Dict
  "msgtype" "link"
  "link" (coll.Dict
    "title" (tmpl.Exec "default.title" . )
    "text" (tmpl.Exec "default.message" . )
    "messageUrl" (print "dingtalk://dingtalkclient/page/link?pc_slide=false&url=" ( $url | urlquery))
  )
  | data.ToJSONPretty " "}}`,
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1"},
						GeneratorURL: "http://localhost/generator",
					},
				},
			},
			expMsg: `
{
 "link": {
  "messageUrl": "dingtalk://dingtalkclient/page/link?pc_slide=false\u0026url=http%3A%2F%2Flocalhost%2Falerting%2Flist",
  "text": "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/generator\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana\u0026matcher=alertname%3Dalert1\u0026matcher=lbl1%3Dval1\n",
  "title": "[FIRING:1]  (val1)"
 },
 "msgtype": "link"
}
`,
			expMsgError: nil,
		},
		{
			name: "kafka-esque payload",
			settings: Config{
				URL:        "http://localhost/test",
				HTTPMethod: http.MethodPost,
				Payload: CustomPayload{
					Template: `{{ coll.Dict
  "type" "JSON"
  "data" (coll.Dict
    "description" (tmpl.Exec "default.title" . )
    "details" (tmpl.Exec "default.message" . )
    "client" "Grafana"
    "client_url" ( print .ExternalURL  "/alerting/list")
    "alert_state" .Status
    "incident_key" .GroupKey
  )
  | data.ToJSONPretty " " }}`,
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1"},
						GeneratorURL: "http://localhost/generator",
					},
				},
			},
			expMsg: `
{
 "data": {
  "alert_state": "firing",
  "client": "Grafana",
  "client_url": "http://localhost/alerting/list",
  "description": "[FIRING:1]  (val1)",
  "details": "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/generator\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana\u0026matcher=alertname%3Dalert1\u0026matcher=lbl1%3Dval1\n",
  "incident_key": "alertname"
 },
 "type": "JSON"
}
`,
			expMsgError: nil,
		},
		{
			name: "empty payload",
			settings: Config{
				URL:        "http://localhost/test",
				HTTPMethod: http.MethodPost,
				Payload: CustomPayload{
					Template: `{{- print "" -}}`,
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1"},
						GeneratorURL: "http://localhost/generator",
					},
				},
			},
			expPlaintext: true,
			expMsg:       ``,
			expMsgError:  nil,
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
				orgID:    orgID,
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
			if c.expPlaintext {
				require.Equal(t, c.expMsg, webhookSender.Webhook.Body)
			} else {
				require.JSONEq(t, c.expMsg, webhookSender.Webhook.Body)
			}
		})
	}
}
