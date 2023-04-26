package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
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

func TestNotify(t *testing.T) {
	tmpl := templates.ForTests(t)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL
	appVersion := fmt.Sprintf("%d.0.0", rand.Uint32())

	cases := []struct {
		name        string
		settings    Config
		alerts      []*types.Alert
		expMsg      map[string]interface{}
		expMsgError error
	}{
		{
			name: "Default config with one alert",
			settings: Config{
				Title:              templates.DefaultMessageTitleEmbed,
				Message:            templates.DefaultMessageEmbed,
				AvatarURL:          "",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: false,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"content": "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				"embeds": []interface{}{map[string]interface{}{
					"color": 1.4037554e+07,
					"footer": map[string]interface{}{
						"icon_url": "https://grafana.com/static/assets/img/fav32.png",
						"text":     "Grafana v" + appVersion,
					},
					"title": "[FIRING:1]  (val1)",
					"url":   "http://localhost/alerting/list",
					"type":  "rich",
				}},
				"username": "Grafana",
			},
			expMsgError: nil,
		},
		{
			name: "Default config with one alert and custom title",
			settings: Config{
				Title:              "Alerts firing: {{ len .Alerts.Firing }}",
				Message:            templates.DefaultMessageEmbed,
				AvatarURL:          "",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: false,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"content": "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				"embeds": []interface{}{map[string]interface{}{
					"color": 1.4037554e+07,
					"footer": map[string]interface{}{
						"icon_url": "https://grafana.com/static/assets/img/fav32.png",
						"text":     "Grafana v" + appVersion,
					},
					"title": "Alerts firing: 1",
					"url":   "http://localhost/alerting/list",
					"type":  "rich",
				}},
				"username": "Grafana",
			},
			expMsgError: nil,
		},
		{
			name: "Missing field in template",
			settings: Config{
				Title:              templates.DefaultMessageTitleEmbed,
				Message:            "I'm a custom template {{ .NotAField }} bad template",
				AvatarURL:          "https://grafana.com/static/assets/img/fav32.png",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: false,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"avatar_url": "https://grafana.com/static/assets/img/fav32.png",
				"content":    "I'm a custom template ",
				"embeds": []interface{}{map[string]interface{}{
					"color": 1.4037554e+07,
					"footer": map[string]interface{}{
						"icon_url": "https://grafana.com/static/assets/img/fav32.png",
						"text":     "Grafana v" + appVersion,
					},
					"title": "[FIRING:1]  (val1)",
					"url":   "http://localhost/alerting/list",
					"type":  "rich",
				}},
				"username": "Grafana",
			},
			expMsgError: nil,
		},
		{
			name: "Invalid message template",
			settings: Config{
				Title:              templates.DefaultMessageTitleEmbed,
				Message:            "{{ template \"invalid.template\" }}",
				AvatarURL:          "https://grafana.com/static/assets/img/fav32.png",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: false,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"avatar_url": "https://grafana.com/static/assets/img/fav32.png",
				"content":    "",
				"embeds": []interface{}{map[string]interface{}{
					"color": 1.4037554e+07,
					"footer": map[string]interface{}{
						"icon_url": "https://grafana.com/static/assets/img/fav32.png",
						"text":     "Grafana v" + appVersion,
					},
					"title": "[FIRING:1]  (val1)",
					"url":   "http://localhost/alerting/list",
					"type":  "rich",
				}},
				"username": "Grafana",
			},
			expMsgError: nil,
		},
		{
			name: "Invalid avatar URL template",
			settings: Config{
				Title:              templates.DefaultMessageTitleEmbed,
				Message:            "valid message",
				AvatarURL:          "{{ invalid } }}",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: false,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"avatar_url": "{{ invalid } }}",
				"content":    "valid message",
				"embeds": []interface{}{map[string]interface{}{
					"color": 1.4037554e+07,
					"footer": map[string]interface{}{
						"icon_url": "https://grafana.com/static/assets/img/fav32.png",
						"text":     "Grafana v" + appVersion,
					},
					"title": "[FIRING:1]  (val1)",
					"url":   "http://localhost/alerting/list",
					"type":  "rich",
				}},
				"username": "Grafana",
			},
			expMsgError: nil,
		},
		{
			name: "Invalid URL template",
			settings: Config{
				Title:              templates.DefaultMessageTitleEmbed,
				Message:            "valid message",
				AvatarURL:          "https://grafana.com/static/assets/img/fav32.png",
				WebhookURL:         "http://localhost?q={{invalid }}}",
				UseDiscordUsername: false,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"avatar_url": "https://grafana.com/static/assets/img/fav32.png",
				"content":    "valid message",
				"embeds": []interface{}{map[string]interface{}{
					"color": 1.4037554e+07,
					"footer": map[string]interface{}{
						"icon_url": "https://grafana.com/static/assets/img/fav32.png",
						"text":     "Grafana v" + appVersion,
					},
					"title": "[FIRING:1]  (val1)",
					"url":   "http://localhost/alerting/list",
					"type":  "rich",
				}},
				"username": "Grafana",
			},
			expMsgError: nil,
		},
		{
			name: "Custom config with multiple alerts",
			settings: Config{
				Title:              templates.DefaultMessageTitleEmbed,
				Message:            "{{ len .Alerts.Firing }} alerts are firing, {{ len .Alerts.Resolved }} are resolved",
				AvatarURL:          "https://grafana.com/static/assets/img/fav32.png",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: false,
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
				"avatar_url": "https://grafana.com/static/assets/img/fav32.png",
				"content":    "2 alerts are firing, 0 are resolved",
				"embeds": []interface{}{map[string]interface{}{
					"color": 1.4037554e+07,
					"footer": map[string]interface{}{
						"icon_url": "https://grafana.com/static/assets/img/fav32.png",
						"text":     "Grafana v" + appVersion,
					},
					"title": "[FIRING:2]  ",
					"url":   "http://localhost/alerting/list",
					"type":  "rich",
				}},
				"username": "Grafana",
			},
			expMsgError: nil,
		},
		{
			name: "Default config with one alert, use default discord username",
			settings: Config{
				Title:              templates.DefaultMessageTitleEmbed,
				Message:            templates.DefaultMessageEmbed,
				AvatarURL:          "",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: true,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"content": "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				"embeds": []interface{}{map[string]interface{}{
					"color": 1.4037554e+07,
					"footer": map[string]interface{}{
						"icon_url": "https://grafana.com/static/assets/img/fav32.png",
						"text":     "Grafana v" + appVersion,
					},
					"title": "[FIRING:1]  (val1)",
					"url":   "http://localhost/alerting/list",
					"type":  "rich",
				}},
			},
			expMsgError: nil,
		},
		{
			name: "Should truncate too long messages",
			settings: Config{
				Title:              templates.DefaultMessageTitleEmbed,
				Message:            strings.Repeat("Y", discordMaxMessageLen+rand.Intn(100)+1),
				AvatarURL:          "",
				WebhookURL:         "http://localhost",
				UseDiscordUsername: true,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"content": strings.Repeat("Y", discordMaxMessageLen-1) + "…",
				"embeds": []interface{}{map[string]interface{}{
					"color": 1.4037554e+07,
					"footer": map[string]interface{}{
						"icon_url": "https://grafana.com/static/assets/img/fav32.png",
						"text":     "Grafana v" + appVersion,
					},
					"title": "[FIRING:1]  (val1)",
					"url":   "http://localhost/alerting/list",
					"type":  "rich",
				}},
			},
			expMsgError: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			webhookSender := receivers.MockNotificationService()
			imageProvider := &images.UnavailableImageProvider{}
			dn := &Notifier{
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
				images:     imageProvider,
				appVersion: appVersion,
			}

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
			ok, err := dn.Notify(ctx, c.alerts...)
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
		})
	}
}
