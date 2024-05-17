package pagerduty

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

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestNotify(t *testing.T) {
	tmpl := templates.ForTests(t)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	hostname, err := os.Hostname()
	require.NoError(t, err)

	cases := []struct {
		name        string
		settings    Config
		alerts      []*types.Alert
		expMsg      *pagerDutyMessage
		expMsgError error
	}{
		{
			name: "Default config with one alert",
			settings: Config{
				Key:       "abcdefgh0123456789",
				Severity:  DefaultSeverity,
				Details:   defaultDetails,
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    hostname,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
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
					Severity:  DefaultSeverity,
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
			name: "should map unknown severity",
			settings: Config{
				Key:       "abcdefgh0123456789",
				Severity:  "{{ .CommonLabels.severity }}",
				Details:   defaultDetails,
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    hostname,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
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
					Severity:  DefaultSeverity,
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
			settings: Config{
				Key:       "abcdefgh0123456789",
				Severity:  "{{ .CommonLabels.severity }}",
				Details:   defaultDetails,
				Class:     "{{ .CommonLabels.class }}",
				Component: "{{ .CommonLabels.component }}",
				Group:     "{{ .CommonLabels.group }}",
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    "{{ .CommonLabels.source }}",
				Client:    "client-{{ .CommonLabels.source }}",
				ClientURL: "http://localhost:20200/{{ .CommonLabels.group }}",
				URL:       "https://events.pagerduty.com/v2/enqueue",
			},
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
			name: "Should expand custom details",
			settings: Config{
				Key:      "abcdefgh0123456789",
				Severity: "{{ .CommonLabels.severity }}",
				Details: map[string]string{
					"firing":       `{{ template "__text_alert_list" .Alerts.Firing }}`,
					"resolved":     `{{ template "__text_alert_list" .Alerts.Resolved }}`,
					"num_firing":   `{{ .Alerts.Firing | len }}`,
					"num_resolved": `{{ .Alerts.Resolved | len }}`,
					"test-field":   "{{ len .Alerts }}",
					"test-field-2": "{{ len \"abcde\"}}",
				},
				Class:     "{{ .CommonLabels.class }}",
				Component: "{{ .CommonLabels.component }}",
				Group:     "{{ .CommonLabels.group }}",
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    "{{ .CommonLabels.source }}",
				Client:    "client-{{ .CommonLabels.source }}",
				ClientURL: "http://localhost:20200/{{ .CommonLabels.group }}",
				URL:       DefaultURL,
			},
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
						"test-field":   "1",
						"test-field-2": "5",
					},
				},
				Client:    "client-test-source",
				ClientURL: "http://localhost:20200/test-group",
				Links:     []pagerDutyLink{{HRef: "http://localhost", Text: "External URL"}},
			},
			expMsgError: nil,
		},
		{
			name: "Should overwrite default custom details with user-defined ones when keys are duplicated",
			settings: Config{
				Key:      "abcdefgh0123456789",
				Severity: "{{ .CommonLabels.severity }}",
				Details: map[string]string{
					"firing":       `{{ len "abcde" }}`,
					"resolved":     "test value",
					"num_firing":   "{{ .Alerts.Firing | len | eq 100 }}",
					"num_resolved": "just another test value",
				},
				Class:     "{{ .CommonLabels.class }}",
				Component: "{{ .CommonLabels.component }}",
				Group:     "{{ .CommonLabels.group }}",
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    "{{ .CommonLabels.source }}",
				Client:    "client-{{ .CommonLabels.source }}",
				ClientURL: "http://localhost:20200/{{ .CommonLabels.group }}",
				URL:       DefaultURL,
			},
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
						"firing":       "5",
						"resolved":     "test value",
						"num_firing":   "false",
						"num_resolved": "just another test value",
					},
				},
				Client:    "client-test-source",
				ClientURL: "http://localhost:20200/test-group",
				Links:     []pagerDutyLink{{HRef: "http://localhost", Text: "External URL"}},
			},
			expMsgError: nil,
		},
		{
			name: "Default config with one alert and custom summary",
			settings: Config{
				Key:       "abcdefgh0123456789",
				Severity:  DefaultSeverity,
				Details:   defaultDetails,
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   "Alerts firing: {{ len .Alerts.Firing }}",
				Source:    hostname,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
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
					Severity:  DefaultSeverity,
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
			settings: Config{
				Key:       "abcdefgh0123456789",
				Severity:  "warning",
				Details:   defaultDetails,
				Class:     "{{ .Status }}",
				Component: "My Grafana",
				Group:     "my_group",
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    hostname,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
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
			name: "should truncate long summary",
			settings: Config{
				Key:       "abcdefgh0123456789",
				Severity:  DefaultSeverity,
				Details:   defaultDetails,
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   strings.Repeat("1", rand.Intn(100)+1025),
				Source:    hostname,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
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
					Severity:  DefaultSeverity,
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
			name: "Should remove custom details if the payload is too large",
			settings: Config{
				Key:      "abcdefgh0123456789",
				Severity: DefaultSeverity,
				Details: map[string]string{
					"long": strings.Repeat("a", pagerDutyMaxEventSize+1),
				},
				Class:     DefaultClass,
				Component: "Grafana",
				Group:     DefaultGroup,
				Summary:   templates.DefaultMessageTitleEmbed,
				Source:    hostname,
				Client:    DefaultClient,
				ClientURL: "{{ .ExternalURL }}",
				URL:       DefaultURL,
			},
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
					Severity:  DefaultSeverity,
					Class:     "default",
					Component: "Grafana",
					Group:     "default",
					CustomDetails: map[string]string{
						"error": "Custom details have been removed because the original event exceeds the maximum size of 512KB",
					},
				},
				Client:    "Grafana",
				ClientURL: "http://localhost",
				Links:     []pagerDutyLink{{HRef: "http://localhost", Text: "External URL"}},
			},
			expMsgError: nil,
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
			}

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
