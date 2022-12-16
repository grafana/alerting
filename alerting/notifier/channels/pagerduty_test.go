package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/services/secrets/fakes"
	secretsManager "github.com/grafana/grafana/pkg/services/secrets/manager"
)

func TestPagerdutyNotifier(t *testing.T) {
	tmpl := templateForTests(t)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	hostname, err := os.Hostname()
	require.NoError(t, err)

	cases := []struct {
		name         string
		settings     string
		alerts       []*types.Alert
		expMsg       *pagerDutyMessage
		expInitError string
		expMsgError  error
	}{
		{
			name:     "Default config with one alert",
			settings: `{"integrationKey": "abcdefgh0123456789"}`,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &pagerDutyMessage{
				RoutingKey:  "abcdefgh0123456789",
				DedupKey:    "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				EventAction: "trigger",
				Payload: pagerDutyPayload{
					Summary:   "[FIRING:1]  (val1)",
					Source:    hostname,
					Severity:  defaultSeverity,
					Class:     "default",
					Component: "Grafana",
					Group:     "default",
					CustomDetails: map[string]string{
						"firing":       "\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
						"num_firing":   "1",
						"num_resolved": "0",
						"resolved":     "",
					},
				},
				Client:    "Grafana",
				ClientURL: "http://localhost",
				Links:     []pagerDutyLink{{HRef: "http://localhost", Text: "External URL"}},
			},
			expMsgError: nil,
		},
		{
			name:     "should map unknown severity",
			settings: `{"integrationKey": "abcdefgh0123456789", "severity": "{{ .CommonLabels.severity }}"}`,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1", "severity": "invalid-severity"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &pagerDutyMessage{
				RoutingKey:  "abcdefgh0123456789",
				DedupKey:    "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				EventAction: "trigger",
				Payload: pagerDutyPayload{
					Summary:   "[FIRING:1]  (val1 invalid-severity)",
					Source:    hostname,
					Severity:  defaultSeverity,
					Class:     "default",
					Component: "Grafana",
					Group:     "default",
					CustomDetails: map[string]string{
						"firing":       "\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\n - severity = invalid-severity\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&matcher=severity%3Dinvalid-severity\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
						"num_firing":   "1",
						"num_resolved": "0",
						"resolved":     "",
					},
				},
				Client:    "Grafana",
				ClientURL: "http://localhost",
				Links:     []pagerDutyLink{{HRef: "http://localhost", Text: "External URL"}},
			},
			expMsgError: nil,
		},
		{
			name: "Should expand templates in fields",
			settings: `{
				"integrationKey": "abcdefgh0123456789", 
				"severity" : "{{ .CommonLabels.severity }}", 
				"class": "{{ .CommonLabels.class }}",  
				"component": "{{ .CommonLabels.component }}", 
				"group" : "{{ .CommonLabels.group }}", 
				"source": "{{ .CommonLabels.source }}",
				"client": "client-{{ .CommonLabels.source }}",
				"client_url": "http://localhost:20200/{{ .CommonLabels.group }}"
			}`,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1", "severity": "critical", "class": "test-class", "group": "test-group", "component": "test-component", "source": "test-source"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &pagerDutyMessage{
				RoutingKey:  "abcdefgh0123456789",
				DedupKey:    "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				EventAction: "trigger",
				Payload: pagerDutyPayload{
					Summary:   "[FIRING:1]  (test-class test-component test-group val1 critical test-source)",
					Source:    "test-source",
					Severity:  "critical",
					Class:     "test-class",
					Component: "test-component",
					Group:     "test-group",
					CustomDetails: map[string]string{
						"firing":       "\nValue: [no value]\nLabels:\n - alertname = alert1\n - class = test-class\n - component = test-component\n - group = test-group\n - lbl1 = val1\n - severity = critical\n - source = test-source\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=class%3Dtest-class&matcher=component%3Dtest-component&matcher=group%3Dtest-group&matcher=lbl1%3Dval1&matcher=severity%3Dcritical&matcher=source%3Dtest-source\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
						"num_firing":   "1",
						"num_resolved": "0",
						"resolved":     "",
					},
				},
				Client:    "client-test-source",
				ClientURL: "http://localhost:20200/test-group",
				Links:     []pagerDutyLink{{HRef: "http://localhost", Text: "External URL"}},
			},
			expMsgError: nil,
		},
		{
			name:     "Default config with one alert and custom summary",
			settings: `{"integrationKey": "abcdefgh0123456789", "summary": "Alerts firing: {{ len .Alerts.Firing }}"}`,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &pagerDutyMessage{
				RoutingKey:  "abcdefgh0123456789",
				DedupKey:    "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				EventAction: "trigger",
				Payload: pagerDutyPayload{
					Summary:   "Alerts firing: 1",
					Source:    hostname,
					Severity:  defaultSeverity,
					Class:     "default",
					Component: "Grafana",
					Group:     "default",
					CustomDetails: map[string]string{
						"firing":       "\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
						"num_firing":   "1",
						"num_resolved": "0",
						"resolved":     "",
					},
				},
				Client:    "Grafana",
				ClientURL: "http://localhost",
				Links:     []pagerDutyLink{{HRef: "http://localhost", Text: "External URL"}},
			},
			expMsgError: nil,
		}, {
			name: "Custom config with multiple alerts",
			settings: `{
				"integrationKey": "abcdefgh0123456789",
				"severity": "warning",
				"class": "{{ .Status }}",
				"component": "My Grafana",
				"group": "my_group"
			}`,
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
			expMsg: &pagerDutyMessage{
				RoutingKey:  "abcdefgh0123456789",
				DedupKey:    "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				EventAction: "trigger",
				Payload: pagerDutyPayload{
					Summary:   "[FIRING:2]  ",
					Source:    hostname,
					Severity:  "warning",
					Class:     "firing",
					Component: "My Grafana",
					Group:     "my_group",
					CustomDetails: map[string]string{
						"firing":       "\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
						"num_firing":   "2",
						"num_resolved": "0",
						"resolved":     "",
					},
				},
				Client:    "Grafana",
				ClientURL: "http://localhost",
				Links:     []pagerDutyLink{{HRef: "http://localhost", Text: "External URL"}},
			},
			expMsgError: nil,
		},
		{
			name:     "should truncate long summary",
			settings: fmt.Sprintf(`{"integrationKey": "abcdefgh0123456789", "summary": "%s"}`, strings.Repeat("1", rand.Intn(100)+1025)),
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &pagerDutyMessage{
				RoutingKey:  "abcdefgh0123456789",
				DedupKey:    "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				EventAction: "trigger",
				Payload: pagerDutyPayload{
					Summary:   fmt.Sprintf("%s…", strings.Repeat("1", 1023)),
					Source:    hostname,
					Severity:  defaultSeverity,
					Class:     "default",
					Component: "Grafana",
					Group:     "default",
					CustomDetails: map[string]string{
						"firing":       "\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
						"num_firing":   "1",
						"num_resolved": "0",
						"resolved":     "",
					},
				},
				Client:    "Grafana",
				ClientURL: "http://localhost",
				Links:     []pagerDutyLink{{HRef: "http://localhost", Text: "External URL"}},
			},
			expMsgError: nil,
		},
		{
			name:         "Error in initing",
			settings:     `{}`,
			expInitError: `could not find integration key property in settings`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			settingsJSON, err := simplejson.NewJson([]byte(c.settings))
			require.NoError(t, err)
			secureSettings := make(map[string][]byte)
			webhookSender := mockNotificationService()
			secretsService := secretsManager.SetupTestService(t, fakes.NewFakeSecretsStore())
			decryptFn := secretsService.GetDecryptedValue

			fc := FactoryConfig{
				Config: &NotificationChannelConfig{
					Name:           "pageduty_testing",
					Type:           "pagerduty",
					Settings:       settingsJSON,
					SecureSettings: secureSettings,
				},
				NotificationService: webhookSender,
				DecryptFunc:         decryptFn,
				Template:            tmpl,
			}
			pn, err := newPagerdutyNotifier(fc)
			if c.expInitError != "" {
				require.Error(t, err)
				require.Equal(t, c.expInitError, err.Error())
				return
			}
			require.NoError(t, err)

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
			ok, err := pn.Notify(ctx, c.alerts...)
			if c.expMsgError != nil {
				require.False(t, ok)
				require.Error(t, err)
				require.Equal(t, c.expMsgError.Error(), err.Error())
				return
			}
			require.True(t, ok)
			require.NoError(t, err)

			expBody, err := json.Marshal(c.expMsg)
			require.NoError(t, err)

			require.JSONEq(t, string(expBody), webhookSender.Webhook.Body)
		})
	}
}
