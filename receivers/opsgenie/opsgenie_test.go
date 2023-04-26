package opsgenie

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestNotify(t *testing.T) {
	tmpl := templates.ForTests(t)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	cases := []struct {
		name        string
		settings    Config
		alerts      []*types.Alert
		expMsg      string
		expMsgError error
	}{
		{
			name: "Default config with one alert",
			settings: Config{
				APIKey:           "abcdefgh0123456789",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendTags,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: `{
				"alias": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"description": "[FIRING:1]  (val1)\nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				"details": {
					"url": "http://localhost/alerting/list"
				},
				"message": "[FIRING:1]  (val1)",
				"source": "Grafana",
				"tags": ["alertname:alert1", "lbl1:val1"]
			}`,
		},
		{
			name: "Default config with one alert, custom message and description",
			settings: Config{
				APIKey:           "abcdefgh0123456789",
				APIUrl:           DefaultAlertsURL,
				Message:          "test message",
				Description:      "test description",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendTags,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: `{
				"alias": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"description": "test description",
				"details": {
					"url": "http://localhost/alerting/list"
				},
				"message": "test message",
				"source": "Grafana",
				"tags": ["alertname:alert1", "lbl1:val1"]
			}`,
		},
		{
			name: "Default config with one alert, message length > 130",
			settings: Config{
				APIKey:           "abcdefgh0123456789",
				APIUrl:           DefaultAlertsURL,
				Message:          "IyJnsW78xQoiBJ7L7NqASv31JCFf0At3r9KUykqBVxSiC6qkDhvDLDW9VImiFcq0Iw2XwFy5fX4FcbTmlkaZzUzjVwx9VUuokhzqQlJVhWDYFqhj3a5wX0LjyvNQjsqT9WaWJAWOJanwOAWon",
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendTags,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: `{
				"alias": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"description": "[FIRING:1]  (val1)\nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				"details": {
					"url": "http://localhost/alerting/list"
				},
				"message": "IyJnsW78xQoiBJ7L7NqASv31JCFf0At3r9KUykqBVxSiC6qkDhvDLDW9VImiFcq0Iw2XwFy5fX4FcbTmlkaZzUzjVwx9VUuokhzqQlJVhWDYFqhj3a5wX0LjyvNQjsqT9…",
				"source": "Grafana",
				"tags": ["alertname:alert1", "lbl1:val1"]
			}`,
		},
		{
			name: "Default config with one alert, templated message and description",
			settings: Config{
				APIKey:           "abcdefgh0123456789",
				APIUrl:           DefaultAlertsURL,
				Message:          "Firing: {{ len .Alerts.Firing }}",
				Description:      "{{ len .Alerts.Firing }} firing, {{ len .Alerts.Resolved }} resolved.",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendTags,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: `{
				"alias": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"description": "1 firing, 0 resolved.",
				"details": {
					"url": "http://localhost/alerting/list"
				},
				"message": "Firing: 1",
				"source": "Grafana",
				"tags": ["alertname:alert1", "lbl1:val1"]
			}`,
		},
		{
			name: "Default config with one alert and send tags as tags, empty description and message",
			settings: Config{
				APIKey:           "abcdefgh0123456789",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      " ",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendTags,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
			expMsg: `{
				"alias": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"description": "[FIRING:1]  (val1)\nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n",
				"details": {
					"url": "http://localhost/alerting/list"
				},
				"message": "[FIRING:1]  (val1)",
				"source": "Grafana",
				"tags": ["alertname:alert1", "lbl1:val1"]
			}`,
		},
		{
			name: "Default config with one alert and send tags as details",
			settings: Config{
				APIKey:           "abcdefgh0123456789",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendDetails,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
			expMsg: `{
				"alias": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"description": "[FIRING:1]  (val1)\nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n",
				"details": {
					"alertname": "alert1",
					"lbl1": "val1",
					"url": "http://localhost/alerting/list"
				},
				"message": "[FIRING:1]  (val1)",
				"source": "Grafana",
				"tags": []
			}`,
		},
		{
			name: "Custom config with multiple alerts and send tags as both details and tag",
			settings: Config{
				APIKey:           "abcdefgh0123456789",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendBoth,
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
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
			expMsg: `{
				"alias": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733",
				"description": "[FIRING:2]  \nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
				"details": {
					"alertname": "alert1",
					"url": "http://localhost/alerting/list"
				},
				"message": "[FIRING:2]  ",
				"source": "Grafana",
				"tags": ["alertname:alert1"]
			}`,
			expMsgError: nil,
		},
		{
			name: "Resolved is not sent when auto close is false",
			settings: Config{
				APIKey:           "abcdefgh0123456789",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        false,
				OverridePriority: true,
				SendTagsAs:       SendBoth,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
						EndsAt:      time.Now().Add(-1 * time.Minute),
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			webhookSender := receivers.MockNotificationService()
			webhookSender.Webhook.Body = "<not-sent>"

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

			if c.expMsg == "" {
				// No notification was expected.
				require.Equal(t, "<not-sent>", webhookSender.Webhook.Body)
			} else {
				require.JSONEq(t, c.expMsg, webhookSender.Webhook.Body)
			}
		})
	}
}
