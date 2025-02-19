package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/models"
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
			imageProvider := &images.UnavailableProvider{}
			dn := &Notifier{
				Base:       &receivers.Base{},
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

func TestNotify_WithImages(t *testing.T) {
	imageWithURL := images.Image{
		URL: "https://www.example.com/test-image-1.jpg",
		RawData: func(_ context.Context) (images.ImageContent, error) {
			return images.ImageContent{}, images.ErrImageNotFound
		},
	}
	imageWithoutURLContent := images.ImageContent{
		Name:    "test-image-2.jpg",
		Content: []byte("test bytes"),
	}
	imageWithoutURL := images.Image{
		URL: "",
		RawData: func(_ context.Context) (images.ImageContent, error) {
			return imageWithoutURLContent, nil
		},
	}

	imageProvider := images.NewTokenProvider(images.NewFakeTokenStoreFromImages(map[string]*images.Image{
		"test-token-url":    &imageWithURL,
		"test-token-no-url": &imageWithoutURL,
	}), &logging.FakeLogger{})
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
		expBytes    []byte
	}{
		{
			name: "Default config with one alert, one image with URL",
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
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", models.ImageTokenAnnotation: model.LabelValue("test-token-url")},
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
				},
					map[string]interface{}{
						"image": map[string]interface{}{
							"url": imageWithURL.URL,
						},
						"title": "alert1",
						"color": 1.4037554e+07,
					}},
				"username": "Grafana",
			},
			expMsgError: nil,
		},
		{
			name: "Default config with one alert, one image without URL",
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
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", models.ImageTokenAnnotation: model.LabelValue("test-token-no-url")},
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
				},
					map[string]interface{}{
						"image": map[string]interface{}{
							"url": "attachment://" + imageWithoutURLContent.Name,
						},
						"title": "alert1",
						"color": 1.4037554e+07,
					}},
				"username": "Grafana",
			},
			expMsgError: nil,
			expBytes:    imageWithoutURLContent.Content,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(tt *testing.T) {
			webhookSender := receivers.MockNotificationService()
			dn := &Notifier{
				Base:       &receivers.Base{},
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
			require.NoError(tt, err)
			require.True(tt, ok)

			expBody, err := json.Marshal(c.expMsg)
			require.NoError(tt, err)

			mediaType, params, err := mime.ParseMediaType(webhookSender.Webhook.ContentType)
			require.NoError(tt, err)
			require.Equal(tt, "multipart/form-data", mediaType)

			reader := multipart.NewReader(strings.NewReader(webhookSender.Webhook.Body), params["boundary"])
			part, err := reader.NextPart()
			require.NoError(tt, err)
			require.Equal(tt, "payload_json", part.FormName())

			buf := bytes.Buffer{}
			_, err = buf.ReadFrom(part)
			require.NoError(tt, err)
			require.JSONEq(tt, string(expBody), buf.String())

			if c.expBytes != nil {
				buf.Reset()
				part, err = reader.NextPart()
				require.NoError(tt, err)

				_, err = buf.ReadFrom(part)
				require.NoError(tt, err)
				require.Equal(tt, c.expBytes, buf.Bytes())
			}
		})
	}

	t.Run("embed quota should be considered", func(tt *testing.T) {
		config := Config{
			Title:              templates.DefaultMessageTitleEmbed,
			Message:            templates.DefaultMessageEmbed,
			AvatarURL:          "",
			WebhookURL:         "http://localhost",
			UseDiscordUsername: false,
		}

		tokenStore := images.NewFakeTokenStore(15)
		imageProvider := images.NewTokenProvider(tokenStore, &logging.FakeLogger{})

		// Create 15 alerts with an image each, Discord's embed limit is 10, and we should be using a maximum of 9 for images.
		var alerts []*types.Alert
		for token := range tokenStore.Images {
			alertName := token
			alert := types.Alert{
				Alert: model.Alert{
					Labels:      model.LabelSet{"alertname": model.LabelValue(alertName), "lbl1": "val"},
					Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", models.ImageTokenAnnotation: model.LabelValue(token)},
				}}

			alerts = append(alerts, &alert)
		}
		// Sort alerts to ensure the expected images are deterministic.
		slices.SortFunc(alerts, func(a, b *types.Alert) int {
			return strings.Compare(a.Name(), b.Name())
		})

		expEmbeds := []interface{}{
			map[string]interface{}{
				"color": 1.4037554e+07,
				"footer": map[string]interface{}{
					"icon_url": "https://grafana.com/static/assets/img/fav32.png",
					"text":     "Grafana v" + appVersion,
				},
				"title": "[FIRING:15]  ",
				"url":   "http://localhost/alerting/list",
				"type":  "rich",
			}}

		for i := 0; i < 9; i++ {
			alert := alerts[i]
			imageEmbed := map[string]interface{}{
				"image": map[string]interface{}{
					"url": tokenStore.Images[alert.Name()].URL,
				},
				"title": alert.Name(),
				"color": 1.4037554e+07,
			}
			expEmbeds = append(expEmbeds, imageEmbed)
		}

		webhookSender := receivers.MockNotificationService()
		dn := &Notifier{
			Base:       &receivers.Base{},
			log:        &logging.FakeLogger{},
			ns:         webhookSender,
			tmpl:       tmpl,
			settings:   config,
			images:     imageProvider,
			appVersion: appVersion,
		}

		ctx := notify.WithGroupKey(context.Background(), "alertname")
		ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
		ok, err := dn.Notify(ctx, alerts...)
		require.NoError(tt, err)
		require.True(tt, ok)

		mediaType, params, err := mime.ParseMediaType(webhookSender.Webhook.ContentType)
		require.NoError(tt, err)
		require.Equal(tt, "multipart/form-data", mediaType)

		reader := multipart.NewReader(strings.NewReader(webhookSender.Webhook.Body), params["boundary"])
		form, err := reader.ReadForm(32 << 20) // 32MB
		require.NoError(tt, err)

		payload, ok := form.Value["payload_json"]
		require.True(tt, ok)

		var payloadMap map[string]interface{}
		require.NoError(tt, json.Unmarshal([]byte(payload[0]), &payloadMap))

		embeds, ok := payloadMap["embeds"]
		require.True(tt, ok)
		require.Len(tt, embeds, 10)
		require.Equal(tt, expEmbeds, embeds)
	})
}
