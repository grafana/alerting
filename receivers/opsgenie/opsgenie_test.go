package opsgenie

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
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

	ctx := notify.WithGroupKey(context.Background(), "alertname")
	ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
	key, err := notify.ExtractGroupKey(ctx)
	require.NoError(t, err)
	groupKeyHash := key.Hash()

	cases := []struct {
		name                  string
		disableResolveMessage bool
		settings              Config
		alerts                []*types.Alert
		expURL                string
		expMsg                string
		expMsgError           error
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
			expMsg: fmt.Sprintf(`{
				"alias": "%s",
				"description": "[FIRING:1]  (val1)\nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				"details": {
					"url": "http://localhost/alerting/list"
				},
				"message": "[FIRING:1]  (val1)",
				"source": "Grafana",
				"tags": ["alertname:alert1", "lbl1:val1"]
			}`, groupKeyHash),
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
			expMsg: fmt.Sprintf(`{
				"alias": "%s",
				"description": "test description",
				"details": {
					"url": "http://localhost/alerting/list"
				},
				"message": "test message",
				"source": "Grafana",
				"tags": ["alertname:alert1", "lbl1:val1"]
			}`, groupKeyHash),
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
			expMsg: fmt.Sprintf(`{
				"alias": "%s",
				"description": "[FIRING:1]  (val1)\nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				"details": {
					"url": "http://localhost/alerting/list"
				},
				"message": "IyJnsW78xQoiBJ7L7NqASv31JCFf0At3r9KUykqBVxSiC6qkDhvDLDW9VImiFcq0Iw2XwFy5fX4FcbTmlkaZzUzjVwx9VUuokhzqQlJVhWDYFqhj3a5wX0LjyvNQjsqT9…",
				"source": "Grafana",
				"tags": ["alertname:alert1", "lbl1:val1"]
			}`, groupKeyHash),
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
			expMsg: fmt.Sprintf(`{
				"alias": "%s",
				"description": "1 firing, 0 resolved.",
				"details": {
					"url": "http://localhost/alerting/list"
				},
				"message": "Firing: 1",
				"source": "Grafana",
				"tags": ["alertname:alert1", "lbl1:val1"]
			}`, groupKeyHash),
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
			expMsg: fmt.Sprintf(`{
				"alias": "%s",
				"description": "[FIRING:1]  (val1)\nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval1\n",
				"details": {
					"url": "http://localhost/alerting/list"
				},
				"message": "[FIRING:1]  (val1)",
				"source": "Grafana",
				"tags": ["alertname:alert1", "lbl1:val1"]
			}`, groupKeyHash),
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
			expMsg: fmt.Sprintf(`{
				"alias": "%s",
				"description": "[FIRING:1]  (val1)\nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval1\n",
				"details": {
					"alertname": "alert1",
					"lbl1": "val1",
					"url": "http://localhost/alerting/list"
				},
				"message": "[FIRING:1]  (val1)",
				"source": "Grafana",
				"tags": []
			}`, groupKeyHash),
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
			expMsg: fmt.Sprintf(`{
				"alias": "%s",
				"description": "[FIRING:2]  \nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval2\n",
				"details": {
					"alertname": "alert1",
					"url": "http://localhost/alerting/list"
				},
				"message": "[FIRING:2]  ",
				"source": "Grafana",
				"tags": ["alertname:alert1"]
			}`, groupKeyHash),
			expMsgError: nil,
		},
		{
			name: "Config with responders",
			settings: Config{
				APIKey:           "abcdefgh0123456789",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendBoth,
				Responders: []MessageResponder{
					{
						Name: "Test User",
						Type: "user",
					},
					{
						Name: "{{ .CommonAnnotations.user }}",
						Type: "{{ .CommonAnnotations.type }}",
					},
					{
						ID:   "{{ .CommonAnnotations.user }}",
						Type: "user",
					},
					{
						Username: "{{ .CommonAnnotations.user }}",
						Type:     "team",
					},
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"user": "test", "type": "team"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"user": "test", "type": "team"},
					},
				},
			},
			expMsg: fmt.Sprintf(`{
				"alias": "%s",
				"description": "[FIRING:2]  \nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - type = team\n - user = test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - type = team\n - user = test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval2\n",
				"details": {
					"alertname": "alert1",
					"url": "http://localhost/alerting/list"
				},
				"message": "[FIRING:2]  ",
				"source": "Grafana",
				"tags": ["alertname:alert1"],
                "responders": [
                    {
                        "name": "Test User",
						"type": "user"
					},
					{
                        "name": "test",
						"type": "team"
					},
					{
                        "id": "test",
						"type": "user"
					},
					{
                        "username": "test",
						"type": "team"
					}
				]
			}`, groupKeyHash),
			expMsgError: nil,
		},
		{
			name: "Config with teams responders should be exploded",
			settings: Config{
				APIKey:           "abcdefgh0123456789",
				APIUrl:           DefaultAlertsURL,
				Message:          templates.DefaultMessageTitleEmbed,
				Description:      "",
				AutoClose:        true,
				OverridePriority: true,
				SendTagsAs:       SendBoth,
				Responders: []MessageResponder{
					{
						Name: "team1,team2,{{ .CommonAnnotations.user }}",
						Type: "teams",
					},
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"user": "test", "type": "team"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"user": "test", "type": "team"},
					},
				},
			},
			expMsg: fmt.Sprintf(`{
				"alias": "%s",
				"description": "[FIRING:2]  \nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - type = team\n - user = test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - type = team\n - user = test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval2\n",
				"details": {
					"alertname": "alert1",
					"url": "http://localhost/alerting/list"
				},
				"message": "[FIRING:2]  ",
				"source": "Grafana",
				"tags": ["alertname:alert1"],
                "responders": [
                    {
                        "name": "team1",
						"type": "team"
					},
					{
                        "name": "team2",
						"type": "team"
					},
					{
                        "name": "test",
						"type": "team"
					}
				]
			}`, groupKeyHash),
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
		{
			name: "Resolved is sent when auto close is true",
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
						EndsAt:      time.Now().Add(-1 * time.Minute),
					},
				},
			},
			expURL: DefaultAlertsURL + "/" + groupKeyHash + "/close?identifierType=alias",
			expMsg: `{"source":"Grafana"}`,
		},
		{
			name:                  "Auto close is ignored when DisableResolveSent is true",
			disableResolveMessage: true,
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
						EndsAt:      time.Now().Add(-1 * time.Minute),
					},
				},
			},
		},
		{
			name: "Should not auto-close if at least one alert is firing",
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
						EndsAt:      time.Now().Add(-1 * time.Minute),
					},
				},
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2"},
					},
				},
			},
			expMsg: fmt.Sprintf(`{
				"alias": "%s",
				"description": "[FIRING:1]  \nhttp://localhost/alerting/list\n\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval2\n\n\n**Resolved**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%%3Dalert1&matcher=lbl1%%3Dval1\n",
				"details": {
					"alertname": "alert1",
					"url": "http://localhost/alerting/list"
				},
				"message": "[FIRING:1]  ",
				"source": "Grafana",
				"tags": ["alertname:alert1"]
			}`, groupKeyHash),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			webhookSender := receivers.MockNotificationService()
			webhookSender.Webhook.URL = DefaultAlertsURL
			webhookSender.Webhook.Body = "<not-sent>"

			pn := &Notifier{
				Base: &receivers.Base{
					Name:                  "",
					Type:                  "",
					UID:                   "",
					DisableResolveMessage: c.disableResolveMessage,
				},
				log:      &logging.FakeLogger{},
				ns:       webhookSender,
				tmpl:     tmpl,
				settings: c.settings,
				images:   &images.UnavailableProvider{},
			}

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
				assert.Equal(t, "<not-sent>", webhookSender.Webhook.Body)
			} else {
				assert.JSONEq(t, c.expMsg, webhookSender.Webhook.Body)
			}
			if c.expURL == "" {
				assert.Equal(t, DefaultAlertsURL, webhookSender.Webhook.URL)
			} else {
				assert.Equal(t, c.expURL, webhookSender.Webhook.URL)
			}
		})
	}
}
