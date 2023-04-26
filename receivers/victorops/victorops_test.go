package victorops

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"testing"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	images2 "github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestNotify(t *testing.T) {
	tmpl := templates.ForTests(t)

	images := images2.NewFakeProvider(2)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL
	version := fmt.Sprintf("%d.0.0", rand.Uint64())

	cases := []struct {
		name        string
		settings    Config
		alerts      []*types.Alert
		expMsg      map[string]interface{}
		expMsgError error
	}{
		{
			name: "A single alert with image",
			settings: Config{
				URL:         "http://localhost",
				MessageType: DefaultMessageType,
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"alert_url":           "http://localhost/alerting/list",
				"entity_display_name": "[FIRING:1]  (val1)",
				"entity_id":           "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"image_url":           "https://www.example.com/test-image-1.jpg",
				"message_type":        "CRITICAL",
				"monitoring_tool":     "Grafana v" + version,
				"state_message":       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
			},
			expMsgError: nil,
		}, {
			name: "Multiple alerts with images",
			settings: Config{
				URL:         "http://localhost",
				MessageType: DefaultMessageType,
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__alertImageToken__": "test-image-1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2", "__alertImageToken__": "test-image-2"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"alert_url":           "http://localhost/alerting/list",
				"entity_display_name": "[FIRING:2]  ",
				"entity_id":           "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"image_url":           "https://www.example.com/test-image-1.jpg",
				"message_type":        "CRITICAL",
				"monitoring_tool":     "Grafana v" + version,
				"state_message":       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
			},
			expMsgError: nil,
		}, {
			name: "Custom message",
			settings: Config{
				URL:         "http://localhost",
				MessageType: "Alerts firing: {{ len .Alerts.Firing }}",
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
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
			expMsg: map[string]interface{}{
				"alert_url":           "http://localhost/alerting/list",
				"entity_display_name": "[FIRING:2]  ",
				"entity_id":           "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"message_type":        "ALERTS FIRING: 2",
				"monitoring_tool":     "Grafana v" + version,
				"state_message":       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
			},
			expMsgError: nil,
		}, {
			name: "Custom title and description",
			settings: Config{
				URL:         "http://localhost",
				MessageType: DefaultMessageType,
				Title:       "Alerts firing: {{ len .Alerts.Firing }}",
				Description: "customDescription",
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
			expMsg: map[string]interface{}{
				"alert_url":           "http://localhost/alerting/list",
				"entity_display_name": "Alerts firing: 2",
				"entity_id":           "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"message_type":        "CRITICAL",
				"monitoring_tool":     "Grafana v" + version,
				"state_message":       "customDescription",
			},
			expMsgError: nil,
		}, {
			name: "Missing field in template",
			settings: Config{
				URL:         "http://localhost",
				MessageType: "custom template {{ .NotAField }} bad template",
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
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
			expMsg: map[string]interface{}{
				"alert_url":           "http://localhost/alerting/list",
				"entity_display_name": "",
				"entity_id":           "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"message_type":        "CUSTOM TEMPLATE ",
				"monitoring_tool":     "Grafana v" + version,
				"state_message":       "",
			},
			expMsgError: nil,
		}, {
			name: "Invalid template",
			settings: Config{
				URL:         "http://localhost",
				MessageType: "custom template {{ {.NotAField }} bad template",
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
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
			expMsg: map[string]interface{}{
				"alert_url":           "http://localhost/alerting/list",
				"entity_display_name": "",
				"entity_id":           "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"message_type":        "CRITICAL",
				"monitoring_tool":     "Grafana v" + version,
				"state_message":       "",
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
				log:        &logging.FakeLogger{},
				ns:         webhookSender,
				tmpl:       tmpl,
				settings:   c.settings,
				images:     images,
				appVersion: version,
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
			require.NoError(t, err)
			require.True(t, ok)

			require.NotEmpty(t, webhookSender.Webhook.URL)

			// Remove the non-constant timestamp
			data := make(map[string]interface{})
			err = json.Unmarshal([]byte(webhookSender.Webhook.Body), &data)
			require.NoError(t, err)
			delete(data, "timestamp")
			b, err := json.Marshal(data)
			require.NoError(t, err)
			body := string(b)

			expJSON, err := json.Marshal(c.expMsg)
			require.NoError(t, err)
			require.JSONEq(t, string(expJSON), body)
		})
	}
}
