package telegram

import (
	"context"
	"net/url"
	"strings"
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
	images := images2.NewFakeProviderWithFile(t, 2)
	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	cases := []struct {
		name        string
		settings    Config
		alerts      []*types.Alert
		expMsg      map[string]string
		expMsgError error
	}{
		{
			name: "A single alert with default template",
			settings: Config{
				BotToken:              "abcdefgh0123456789",
				ChatID:                "someid",
				MessageThreadID:       "threadid",
				Message:               templates.DefaultMessageEmbed,
				ParseMode:             "Markdown",
				DisableWebPagePreview: true,
				ProtectContent:        true,
				DisableNotifications:  true,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
						GeneratorURL: "a URL",
					},
				},
			},
			expMsg: map[string]string{
				"message_thread_id":        "threadid",
				"parse_mode":               "Markdown",
				"text":                     "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: a URL\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				"disable_web_page_preview": "true",
				"protect_content":          "true",
				"disable_notification":     "true",
			},
			expMsgError: nil,
		}, {
			name: "Multiple alerts with custom template",
			settings: Config{
				BotToken:  "abcdefgh0123456789",
				ChatID:    "someid",
				Message:   "__Custom Firing__\n{{len .Alerts.Firing}} Firing\n{{ template \"__text_alert_list\" .Alerts.Firing }}",
				ParseMode: DefaultTelegramParseMode,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__alertImageToken__": "test-image-1"},
						GeneratorURL: "a URL",
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2", "__alertImageToken__": "test-image-2"},
					},
				},
			},
			expMsg: map[string]string{
				"parse_mode": "HTML",
				"text":       "__Custom Firing__\n2 Firing\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: a URL\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
			},
			expMsgError: nil,
		}, {
			name: "Truncate long message",
			settings: Config{
				BotToken:  "abcdefgh0123456789",
				ChatID:    "someid",
				Message:   "{{ .CommonLabels.alertname }}",
				ParseMode: DefaultTelegramParseMode,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels: model.LabelSet{"alertname": model.LabelValue(strings.Repeat("1", 4097))},
					},
				},
			},
			expMsg: map[string]string{
				"parse_mode": "HTML",
				"text":       strings.Repeat("1", 4096-1) + "…",
			},
			expMsgError: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			notificationService := receivers.MockNotificationService()

			n := &Notifier{
				Base: &receivers.Base{
					Name:                  "",
					Type:                  "",
					UID:                   "",
					DisableResolveMessage: false,
				},
				log:      &logging.FakeLogger{},
				ns:       notificationService,
				tmpl:     tmpl,
				settings: c.settings,
				images:   images,
			}

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
			ok, err := n.Notify(ctx, c.alerts...)
			require.NoError(t, err)
			require.True(t, ok)

			msg, err := n.buildTelegramMessage(ctx, c.alerts)
			if c.expMsgError != nil {
				require.Error(t, err)
				require.Equal(t, c.expMsgError.Error(), err.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expMsg, msg)
		})
	}
}
