package telegram

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

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
	image1Name := "test-image-1.jpg"
	image2Name := "test-image-2.jpg"

	cases := []struct {
		name        string
		settings    Config
		alerts      []*types.Alert
		expMsg      []map[string]string
		expMsgError error
		expImages   int
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
			expMsg: []map[string]string{{
				"chat_id":                  "someid",
				"message_thread_id":        "threadid",
				"parse_mode":               "Markdown",
				"text":                     "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: a URL\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
				"disable_web_page_preview": "true",
				"protect_content":          "true",
				"disable_notification":     "true",
			}, {
				"chat_id":              "someid",
				"message_thread_id":    "threadid",
				"disable_notification": "true",
				"photo":                image1Name,
			}},
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
			expMsg: []map[string]string{{
				"chat_id":    "someid",
				"parse_mode": "HTML",
				"text":       "__Custom Firing__\n2 Firing\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: a URL\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
			}, {
				"chat_id": "someid",
				"photo":   image1Name,
			}, {
				"chat_id": "someid",
				"photo":   image2Name,
			}},
			expMsgError: nil,
			expImages:   2,
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
			expMsg: []map[string]string{{
				"chat_id":    "someid",
				"parse_mode": "HTML",
				"text":       strings.Repeat("1", 4096-1) + "…",
			}},
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
			recoverableErr, err := n.Notify(ctx, c.alerts...)
			if c.expMsgError != nil {
				assert.False(t, recoverableErr)
				require.Error(t, err)
				require.Equal(t, c.expMsgError.Error(), err.Error())
				return
			}
			require.NoError(t, err)

			require.NotEmpty(t, notificationService.WebhookCalls, "webhook expected to be called but it wasn't")
			require.Lenf(t, notificationService.WebhookCalls, len(c.expMsg), "expected %d requests to be made but got %d", len(c.expMsg), len(notificationService.WebhookCalls))

			t.Run("message should go first", func(t *testing.T) {
				msgCmd := notificationService.WebhookCalls[0]
				assert.Equal(t, "https://api.telegram.org/bot"+c.settings.BotToken+"/sendMessage", msgCmd.URL)
			})
			if len(c.expMsg) > 1 {
				t.Run("should send one request per image", func(t *testing.T) {
					for i, call := range notificationService.WebhookCalls[1:] {
						assert.Equalf(t, "https://api.telegram.org/bot"+c.settings.BotToken+"/sendPhoto", call.URL, "request at index %d was expected to be sendPhoto", i)
					}
				})
			}

			for i, call := range notificationService.WebhookCalls {
				if !assert.Containsf(t, call.HTTPHeader, "Content-Type", "request at index %d did not contain Content-Type header", i) {
					continue
				}
				mediaType, params, err := mime.ParseMediaType(call.HTTPHeader["Content-Type"])
				assert.NoError(t, err)
				assert.Equal(t, "multipart/form-data", mediaType)

				reader := multipart.NewReader(strings.NewReader(call.Body), params["boundary"])
				data := map[string]string{}
				for {
					p, err := reader.NextPart()
					if errors.Is(err, io.EOF) {
						break
					}
					require.NoError(t, err)
					slurp, err := io.ReadAll(p)
					require.NoError(t, err)
					fieldName := p.FormName()
					photoName := p.FileName()
					if assert.NotEmpty(t, fieldName) {
						if len(photoName) > 0 {
							assert.NotEmptyf(t, slurp, "Content of the photo %s at in request at index %d is empty but it should not", photoName, i)
							data[fieldName] = photoName
						} else {
							data[fieldName] = string(slurp)
						}
					}
				}
				assert.Equalf(t, c.expMsg[i], data, "form-data at index %d does not match expected one", i)
			}
		})
	}
}
