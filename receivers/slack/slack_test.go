package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

var appVersion = fmt.Sprintf("%d.0.0", rand.Uint32())

func TestNotify_IncomingWebhook(t *testing.T) {
	tests := []struct {
		name            string
		alerts          []*types.Alert
		expectedMessage *slackMessage
		expectedError   string
		settings        Config
	}{{
		name: "Message is sent",
		settings: Config{
			EndpointURL:    APIURL,
			URL:            "https://example.com/hooks/xxxx",
			Token:          "",
			Recipient:      "#test",
			Text:           templates.DefaultMessageEmbed,
			Title:          templates.DefaultMessageTitleEmbed,
			Username:       "Grafana",
			IconEmoji:      ":emoji:",
			IconURL:        "",
			MentionChannel: "",
			MentionUsers:   nil,
			MentionGroups:  nil,
			Color:          templates.DefaultMessageColor,
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations: model.LabelSet{"ann1": "annv1"},
			},
		}},
		expectedMessage: &slackMessage{
			Channel:   "#test",
			Username:  "Grafana",
			IconEmoji: ":emoji:",
			Attachments: []attachment{
				{
					Title:      "[FIRING:1]  (val1)",
					TitleLink:  "http://localhost/alerting/list",
					Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n",
					Fallback:   "[FIRING:1]  (val1)",
					Fields:     nil,
					Footer:     "Grafana v" + appVersion,
					FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
					Color:      "#D63232",
				},
			},
		},
	}, {
		name: "Message is sent with image URL",
		settings: Config{
			EndpointURL:    APIURL,
			URL:            "https://example.com/hooks/xxxx",
			Token:          "",
			Recipient:      "#test",
			Text:           templates.DefaultMessageEmbed,
			Title:          templates.DefaultMessageTitleEmbed,
			Username:       "Grafana",
			IconEmoji:      ":emoji:",
			IconURL:        "",
			MentionChannel: "",
			MentionUsers:   nil,
			MentionGroups:  nil,
			Color:          templates.DefaultMessageColor,
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "image-with-url"},
			},
		}},
		expectedMessage: &slackMessage{
			Channel:   "#test",
			Username:  "Grafana",
			IconEmoji: ":emoji:",
			Attachments: []attachment{
				{
					Title:      "[FIRING:1]  (val1)",
					TitleLink:  "http://localhost/alerting/list",
					Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
					Fallback:   "[FIRING:1]  (val1)",
					Fields:     nil,
					Footer:     "Grafana v" + appVersion,
					FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
					Color:      "#D63232",
					ImageURL:   "https://www.example.com/test.png",
				},
			},
		},
	}, {
		name: "Message is sent and image on local disk is ignored",
		settings: Config{
			EndpointURL:    APIURL,
			URL:            "https://example.com/hooks/xxxx",
			Token:          "",
			Recipient:      "#test",
			Text:           templates.DefaultMessageEmbed,
			Title:          templates.DefaultMessageTitleEmbed,
			Username:       "Grafana",
			IconEmoji:      ":emoji:",
			IconURL:        "",
			MentionChannel: "",
			MentionUsers:   nil,
			MentionGroups:  nil,
			Color:          templates.DefaultMessageColor,
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "image-on-disk"},
			},
		}},
		expectedMessage: &slackMessage{
			Channel:   "#test",
			Username:  "Grafana",
			IconEmoji: ":emoji:",
			Attachments: []attachment{
				{
					Title:      "[FIRING:1]  (val1)",
					TitleLink:  "http://localhost/alerting/list",
					Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
					Fallback:   "[FIRING:1]  (val1)",
					Fields:     nil,
					Footer:     "Grafana v" + appVersion,
					FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
					Color:      "#D63232",
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notifier, recorder, err := setupSlackForTests(t, test.settings)
			require.NoError(t, err)

			ctx := context.Background()
			ctx = notify.WithGroupKey(ctx, "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})

			ok, err := notifier.Notify(ctx, test.alerts...)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				assert.False(t, ok)
			} else {
				assert.NoError(t, err)
				assert.True(t, ok)

				// When sending a notification to an Incoming Webhook there should a single request.
				// This is different from PostMessage where some content, such as images, are sent
				// as replies to the original message
				require.Equal(t, recorder.requestCount, 1)

				// Get the request and check that it's sending to the URL of the Incoming Webhook
				r := recorder.messageRequest
				assert.Equal(t, notifier.settings.URL, r.URL.String())

				// Check that the request contains the expected message
				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				message := slackMessage{}
				require.NoError(t, json.Unmarshal(b, &message))
				for i, v := range message.Attachments {
					// Need to update the ts as these cannot be set in the test definition
					test.expectedMessage.Attachments[i].Ts = v.Ts
				}
				assert.Equal(t, *test.expectedMessage, message)
			}
		})
	}
}

func TestNotify_PostMessage(t *testing.T) {
	tests := []struct {
		name            string
		alerts          []*types.Alert
		expectedMessage *slackMessage
		expectedError   string
		settings        Config
	}{{
		name: "Message is sent",
		settings: Config{
			EndpointURL:    APIURL,
			URL:            APIURL,
			Token:          "1234",
			Recipient:      "#test",
			Text:           templates.DefaultMessageEmbed,
			Title:          templates.DefaultMessageTitleEmbed,
			Username:       "Grafana",
			IconEmoji:      ":emoji:",
			IconURL:        "",
			MentionChannel: "",
			MentionUsers:   nil,
			MentionGroups:  nil,
			Color:          templates.DefaultMessageColor,
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
			},
		}},
		expectedMessage: &slackMessage{
			Channel:   "#test",
			Username:  "Grafana",
			IconEmoji: ":emoji:",
			Attachments: []attachment{
				{
					Title:      "[FIRING:1]  (val1)",
					TitleLink:  "http://localhost/alerting/list",
					Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
					Fallback:   "[FIRING:1]  (val1)",
					Fields:     nil,
					Footer:     "Grafana v" + appVersion,
					FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
					Color:      "#D63232",
				},
			},
		},
	}, {
		name: "Message is sent with a single alert and a GeneratorURL",
		settings: Config{
			EndpointURL:    APIURL,
			URL:            APIURL,
			Token:          "1234",
			Recipient:      "#test",
			Text:           templates.DefaultMessageEmbed,
			Title:          templates.DefaultMessageTitleEmbed,
			Username:       "Grafana",
			IconEmoji:      ":emoji:",
			IconURL:        "",
			MentionChannel: "",
			MentionUsers:   nil,
			MentionGroups:  nil,
			Color:          templates.DefaultMessageColor,
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
				GeneratorURL: "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
			},
		}},
		expectedMessage: &slackMessage{
			Channel:   "#test",
			Username:  "Grafana",
			IconEmoji: ":emoji:",
			Attachments: []attachment{
				{
					Title:      "[FIRING:1]  (val1)",
					TitleLink:  "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
					Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
					Fallback:   "[FIRING:1]  (val1)",
					Fields:     nil,
					Footer:     "Grafana v" + appVersion,
					FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
					Color:      "#D63232",
				},
			},
		},
	}, {
		name: "Message is sent with two firing alerts with different GeneratorURLs",
		settings: Config{
			EndpointURL:    APIURL,
			URL:            APIURL,
			Token:          "1234",
			Recipient:      "#test",
			Text:           templates.DefaultMessageEmbed,
			Title:          "{{ .Alerts.Firing | len }} firing, {{ .Alerts.Resolved | len }} resolved",
			Username:       "Grafana",
			IconEmoji:      ":emoji:",
			IconURL:        "",
			MentionChannel: "",
			MentionUsers:   nil,
			MentionGroups:  nil,
			Color:          templates.DefaultMessageColor,
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations:  model.LabelSet{"ann1": "annv1"},
				GeneratorURL: "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
			},
		}, {
			Alert: model.Alert{
				Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
				Annotations:  model.LabelSet{"ann1": "annv2"},
				GeneratorURL: "http://localhost/alerting/f23a674b-bb6b-46df-8723-1234567test2",
			},
		}},
		expectedMessage: &slackMessage{
			Channel:   "#test",
			Username:  "Grafana",
			IconEmoji: ":emoji:",
			Attachments: []attachment{
				{
					Title:      "2 firing, 0 resolved",
					TitleLink:  "http://localhost/alerting/list",
					Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSource: http://localhost/alerting/f23a674b-bb6b-46df-8723-1234567test2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
					Fallback:   "2 firing, 0 resolved",
					Fields:     nil,
					Footer:     "Grafana v" + appVersion,
					FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
					Color:      "#D63232",
				},
			},
		},
	}, {
		name: "Message is sent with two firing alerts with the same GeneratorURLs",
		settings: Config{
			EndpointURL:    APIURL,
			URL:            APIURL,
			Token:          "1234",
			Recipient:      "#test",
			Text:           templates.DefaultMessageEmbed,
			Title:          "{{ .Alerts.Firing | len }} firing, {{ .Alerts.Resolved | len }} resolved",
			Username:       "Grafana",
			IconEmoji:      ":emoji:",
			IconURL:        "",
			MentionChannel: "",
			MentionUsers:   nil,
			MentionGroups:  nil,
			Color:          templates.DefaultMessageColor,
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations:  model.LabelSet{"ann1": "annv1"},
				GeneratorURL: "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
			},
		}, {
			Alert: model.Alert{
				Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
				Annotations:  model.LabelSet{"ann1": "annv2"},
				GeneratorURL: "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
			},
		}},
		expectedMessage: &slackMessage{
			Channel:   "#test",
			Username:  "Grafana",
			IconEmoji: ":emoji:",
			Attachments: []attachment{
				{
					Title:      "2 firing, 0 resolved",
					TitleLink:  "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
					Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSource: http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
					Fallback:   "2 firing, 0 resolved",
					Fields:     nil,
					Footer:     "Grafana v" + appVersion,
					FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
					Color:      "#D63232",
				},
			},
		},
	}, {
		name: "Message is sent with two firing alerts",
		settings: Config{
			EndpointURL:    APIURL,
			URL:            APIURL,
			Token:          "1234",
			Recipient:      "#test",
			Text:           templates.DefaultMessageEmbed,
			Title:          "{{ .Alerts.Firing | len }} firing, {{ .Alerts.Resolved | len }} resolved",
			Username:       "Grafana",
			IconEmoji:      ":emoji:",
			IconURL:        "",
			MentionChannel: "",
			MentionUsers:   nil,
			MentionGroups:  nil,
			Color:          templates.DefaultMessageColor,
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations: model.LabelSet{"ann1": "annv1"},
			},
		}, {
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
				Annotations: model.LabelSet{"ann1": "annv2"},
			},
		}},
		expectedMessage: &slackMessage{
			Channel:   "#test",
			Username:  "Grafana",
			IconEmoji: ":emoji:",
			Attachments: []attachment{
				{
					Title:      "2 firing, 0 resolved",
					TitleLink:  "http://localhost/alerting/list",
					Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
					Fallback:   "2 firing, 0 resolved",
					Fields:     nil,
					Footer:     "Grafana v" + appVersion,
					FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
					Color:      "#D63232",
				},
			},
		},
	}, {
		name: "Errors in templates, message is sent",
		settings: Config{
			EndpointURL:    APIURL,
			URL:            APIURL,
			Token:          "1234",
			Recipient:      "#test",
			Text:           `{{ template "undefined" . }}`,
			Title:          `{{ template "undefined" . }}`,
			Username:       `{{ template "undefined" . }}`,
			IconEmoji:      `{{ template "undefined" . }}`,
			IconURL:        "",
			MentionChannel: "",
			MentionUsers:   nil,
			MentionGroups:  nil,
			Color:          templates.DefaultMessageColor,
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations: model.LabelSet{"ann1": "annv1"},
			},
		}},
		expectedMessage: &slackMessage{
			Channel:   "#test",
			Username:  "",
			IconEmoji: "",
			Attachments: []attachment{
				{
					Title:      "",
					TitleLink:  "http://localhost/alerting/list",
					Text:       "",
					Fallback:   "",
					Fields:     nil,
					Footer:     "Grafana v" + appVersion,
					FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
					Color:      "",
				},
			},
		},
	}, {
		name: "Message is sent to custom URL",
		settings: Config{
			EndpointURL:    "https://example.com/api",
			URL:            "https://example.com/api",
			Token:          "1234",
			Recipient:      "#test",
			Text:           templates.DefaultMessageEmbed,
			Title:          templates.DefaultMessageTitleEmbed,
			Username:       "Grafana",
			IconEmoji:      ":emoji:",
			IconURL:        "",
			MentionChannel: "",
			MentionUsers:   nil,
			MentionGroups:  nil,
			Color:          templates.DefaultMessageColor,
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations: model.LabelSet{"ann1": "annv1"},
			},
		}},
		expectedMessage: &slackMessage{
			Channel:   "#test",
			Username:  "Grafana",
			IconEmoji: ":emoji:",
			Attachments: []attachment{
				{
					Title:      "[FIRING:1]  (val1)",
					TitleLink:  "http://localhost/alerting/list",
					Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n",
					Fallback:   "[FIRING:1]  (val1)",
					Fields:     nil,
					Footer:     "Grafana v" + appVersion,
					FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
					Color:      "#D63232",
				},
			},
		},
	}, {
		name: "Message is sent to with custom color",
		settings: Config{
			EndpointURL:    APIURL,
			URL:            APIURL,
			Token:          "1234",
			Recipient:      "#test",
			Text:           templates.DefaultMessageEmbed,
			Title:          templates.DefaultMessageTitleEmbed,
			Username:       "Grafana",
			IconEmoji:      ":emoji:",
			IconURL:        "",
			MentionChannel: "",
			MentionUsers:   nil,
			MentionGroups:  nil,
			Color:          `{{ if eq .Status "firing" }}#33a2ff{{ else }}#36a64f{{ end }}`,
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations: model.LabelSet{"ann1": "annv1"},
			},
		}},
		expectedMessage: &slackMessage{
			Channel:   "#test",
			Username:  "Grafana",
			IconEmoji: ":emoji:",
			Attachments: []attachment{
				{
					Title:      "[FIRING:1]  (val1)",
					TitleLink:  "http://localhost/alerting/list",
					Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n",
					Fallback:   "[FIRING:1]  (val1)",
					Fields:     nil,
					Footer:     "Grafana v" + appVersion,
					FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
					Color:      "#33a2ff",
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notifier, recorder, err := setupSlackForTests(t, test.settings)
			require.NoError(t, err)

			ctx := context.Background()
			ctx = notify.WithGroupKey(ctx, "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})

			ok, err := notifier.Notify(ctx, test.alerts...)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				assert.False(t, ok)
			} else {
				assert.NoError(t, err)
				assert.True(t, ok)

				require.Equal(t, recorder.requestCount, 1)

				// Get the request and check that it's sending to the URL
				r := recorder.messageRequest
				assert.Equal(t, notifier.settings.URL, r.URL.String())

				// Check that the request contains the expected message
				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				message := slackMessage{}
				require.NoError(t, json.Unmarshal(b, &message))
				for i, v := range message.Attachments {
					// Need to update the ts as these cannot be set in the test definition
					test.expectedMessage.Attachments[i].Ts = v.Ts
				}
				assert.Equal(t, *test.expectedMessage, message)
			}
		})
	}
}

func TestNotify_PostMessageWithImage(t *testing.T) {
	tests := []struct {
		name                 string
		alerts               []*types.Alert
		expectedMessage      *slackMessage
		expectedImageUploads []CompleteFileUploadRequest
		expectedError        string
		settings             Config
	}{
		{
			name: "Message is sent and image is uploaded",
			settings: Config{
				EndpointURL:    APIURL,
				URL:            APIURL,
				Token:          "1234",
				Recipient:      "#test",
				Text:           templates.DefaultMessageEmbed,
				Title:          templates.DefaultMessageTitleEmbed,
				Username:       "Grafana",
				IconEmoji:      ":emoji:",
				IconURL:        "",
				MentionChannel: "",
				MentionUsers:   nil,
				MentionGroups:  nil,
				Color:          templates.DefaultMessageColor,
			},
			alerts: []*types.Alert{{
				Alert: model.Alert{
					Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1", "__grafana_autogenerated__": "true", "__grafana_receiver__": "slack", "__grafana_route_settings_hash__": "1234"},
					Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "image-on-disk"},
				},
			}},
			expectedMessage: &slackMessage{
				Channel:   "#test",
				Username:  "Grafana",
				IconEmoji: ":emoji:",
				Attachments: []attachment{
					{
						Title:      "[FIRING:1]  (val1)",
						TitleLink:  "http://localhost/alerting/list",
						Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
						Fallback:   "[FIRING:1]  (val1)",
						Fields:     nil,
						Footer:     "Grafana v" + appVersion,
						FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
						Color:      "#D63232",
					},
				},
			},
			expectedImageUploads: []CompleteFileUploadRequest{
				{
					Files: []struct {
						ID string `json:"id"`
					}{
						{ID: "file-id"},
					},
					ChannelID:      "C123ABC456",
					ThreadTs:       "1503435956.000247",
					InitialComment: "*Firing*: alert1, *Labels*: lbl1=val1",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notifier, recorder, err := setupSlackForTests(t, test.settings)
			require.NoError(t, err)

			ctx := context.Background()
			ctx = notify.WithGroupKey(ctx, "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})

			ok, err := notifier.Notify(ctx, test.alerts...)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				assert.False(t, ok)
			} else {
				assert.NoError(t, err)
				assert.True(t, ok)

				// When sending a notification via PostMessage some content, such as images,
				// are sent as replies to the original message
				imageUploadRequestCount := len(test.expectedImageUploads) * 3
				require.Equal(t, recorder.requestCount, 1+imageUploadRequestCount)

				// Get the request and check that it's sending to the URL
				r := recorder.messageRequest
				assert.Equal(t, notifier.settings.URL, r.URL.String())

				// Check that the request contains the expected message
				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				message := slackMessage{}
				require.NoError(t, json.Unmarshal(b, &message))
				for i, v := range message.Attachments {
					// Need to update the ts as these cannot be set in the test definition
					test.expectedMessage.Attachments[i].Ts = v.Ts
				}
				assert.Equal(t, *test.expectedMessage, message)

				tokenHeader := fmt.Sprintf("Bearer %s", test.settings.Token)

				readBody := func(r *http.Request) []byte {
					b, err := io.ReadAll(r.Body)
					assert.NoError(t, err)
					return b
				}

				for i := 0; i < len(test.expectedImageUploads); i++ {
					// check first request is to get the upload URL
					initRequest := recorder.initFileUploadRequests[i]
					assert.Equal(t, "GET", initRequest.Method)
					pathParts := strings.Split(initRequest.URL.EscapedPath(), "/")
					assert.Equal(t, "files.getUploadURLExternal", pathParts[len(pathParts)-1])
					assert.Contains(t, strings.Split(initRequest.Header.Get("Content-Type"), ";"), "application/x-www-form-urlencoded")
					assert.Contains(t, initRequest.URL.Query(), "filename")
					assert.Contains(t, initRequest.URL.Query(), "length")
					assert.Equal(t, tokenHeader, initRequest.Header.Get("Authorization"))
					// check second request is to upload the image
					uploadRequest := recorder.fileUploadRequests[i]
					assert.Equal(t, "POST", uploadRequest.Method)
					assert.NoError(t, uploadRequest.ParseMultipartForm(32<<20))
					assert.Equal(t, "test.png", uploadRequest.MultipartForm.File["filename"][0].Filename)
					assert.Contains(t, strings.Split(uploadRequest.Header.Get("Content-Type"), ";"), "multipart/form-data")
					assert.Equal(t, tokenHeader, uploadRequest.Header.Get("Authorization"))
					// check third request is to finalize the upload
					finalizeRequest := recorder.completeFileUploads[i]
					assert.Equal(t, "POST", finalizeRequest.Method)
					pathParts = strings.Split(finalizeRequest.URL.EscapedPath(), "/")
					assert.Equal(t, "files.completeUploadExternal", pathParts[len(pathParts)-1])
					assert.Contains(t, strings.Split(finalizeRequest.Header.Get("Content-Type"), ";"), "application/json")
					assert.Equal(t, tokenHeader, finalizeRequest.Header.Get("Authorization"))
					var finalizeReqBody CompleteFileUploadRequest
					assert.NoError(t, json.Unmarshal(readBody(finalizeRequest), &finalizeReqBody))
					assert.Equal(t, test.expectedImageUploads[i], finalizeReqBody)
				}
			}
		})
	}
}

// slackRequestRecorder is used in tests to record all requests.
type slackRequestRecorder struct {
	requestCount           int
	messageRequest         *http.Request
	initFileUploadRequests []*http.Request
	fileUploadRequests     []*http.Request
	completeFileUploads    []*http.Request
}

func (s *slackRequestRecorder) recordMessageRequest(_ context.Context, r *http.Request, _ logging.Logger) (slackMessageResponse, error) {
	s.requestCount++
	s.messageRequest = r
	return slackMessageResponse{Ts: "1503435956.000247", Channel: "C123ABC456"}, nil
}

func (s *slackRequestRecorder) recordInitFileUploadRequest(_ context.Context, r *http.Request, _ logging.Logger) (*FileUploadURLResponse, error) {
	s.requestCount++
	s.initFileUploadRequests = append(s.initFileUploadRequests, r)
	return &FileUploadURLResponse{
		FileID:    "file-id",
		UploadURL: "TODO: replace this with some function that actually allows you to return something to test the flow",
	}, nil
}

func (s *slackRequestRecorder) recordFileUploadRequest(_ context.Context, r *http.Request, _ logging.Logger) error {
	s.requestCount++
	s.fileUploadRequests = append(s.fileUploadRequests, r)
	return nil
}

func (s *slackRequestRecorder) recordCompleteFileUpload(_ context.Context, r *http.Request, _ logging.Logger) error {
	s.requestCount++
	s.completeFileUploads = append(s.completeFileUploads, r)
	return nil
}

func setupSlackForTests(t *testing.T, settings Config) (*Notifier, *slackRequestRecorder, error) {
	tmpl := templates.ForTests(t)
	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	images := images.NewTokenProvider(images.NewFakeTokenStoreFromImages(map[string]*images.Image{
		"image-on-disk": {
			RawData: func(_ context.Context) (images.ImageContent, error) {
				return images.ImageContent{
					Name:    "test.png",
					Content: []byte("some image"),
				}, nil
			},
		},
		"image-with-url": {
			URL: "https://www.example.com/test.png",
			RawData: func(_ context.Context) (images.ImageContent, error) {
				return images.ImageContent{}, images.ErrImageNotFound
			},
		},
	},
	), &logging.FakeLogger{})
	notificationService := receivers.MockNotificationService()

	sn := &Notifier{
		Base: &receivers.Base{
			Name:                  "",
			Type:                  "",
			UID:                   "",
			DisableResolveMessage: false,
		},
		log:           &logging.FakeLogger{},
		webhookSender: notificationService,
		tmpl:          tmpl,
		settings:      settings,
		images:        images,
		appVersion:    appVersion,
	}

	sr := &slackRequestRecorder{}
	sn.sendMessageFn = sr.recordMessageRequest
	sn.initFileUploadFn = sr.recordInitFileUploadRequest
	sn.uploadFileFn = sr.recordFileUploadRequest
	sn.completeFileUploadFn = sr.recordCompleteFileUpload

	return sn, sr, nil
}

func TestSendSlackRequest(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		statusCode  int
		contentType string
		expectError bool
	}{
		{
			name: "Example error",
			response: `{
					"ok": false,
					"error": "too_many_attachments"
				}`,
			statusCode:  http.StatusBadRequest,
			contentType: "application/json",
			expectError: true,
		},
		{
			name:        "Non 200 status code, no response body",
			statusCode:  http.StatusMovedPermanently,
			contentType: "",
			expectError: true,
		},
		{
			name: "Success case, normal response body",
			response: `{
				"ok": true,
				"channel": "C1H9RESGL",
				"ts": "1503435956.000247",
				"message": {
					"text": "Here's a message for you",
					"username": "ecto1",
					"bot_id": "B19LU7CSY",
					"attachments": [
						{
							"text": "This is an attachment",
							"id": 1,
							"fallback": "This is an attachment's fallback"
						}
					],
					"type": "message",
					"subtype": "bot_message",
					"ts": "1503435956.000247"
				}
			}`,
			statusCode:  http.StatusOK,
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "No JSON response body",
			statusCode:  http.StatusOK,
			contentType: "application/json",
			expectError: true,
		},
		{
			name:        "No HTML response body",
			statusCode:  http.StatusOK,
			contentType: "text/html",
			expectError: true,
		},
		{
			name:        "Success case, unexpected response body",
			statusCode:  http.StatusOK,
			response:    `{"test": true}`,
			contentType: "application/json",
			expectError: true,
		},
		{
			name:        "Success case, ok: true",
			statusCode:  http.StatusOK,
			response:    `{"ok": true}`,
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "200 status code, error in body",
			statusCode:  http.StatusOK,
			response:    `{"ok": false, "error": "test error"}`,
			contentType: "application/json",
			expectError: true,
		},
		{
			name:        "Success case, HTML ok",
			statusCode:  http.StatusOK,
			response:    "ok",
			contentType: "text/html",
			expectError: false,
		},
		{
			name:        "Success case, text/plain ok",
			statusCode:  http.StatusOK,
			response:    "ok",
			contentType: "text/plain",
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if test.contentType != "" {
					w.Header().Set("Content-Type", test.contentType)
				}
				w.WriteHeader(test.statusCode)
				_, err := w.Write([]byte(test.response))
				require.NoError(tt, err)
			}))
			defer server.Close()
			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			require.NoError(tt, err)

			_, err = sendSlackMessage(context.Background(), req, &logging.FakeLogger{})
			if !test.expectError {
				require.NoError(tt, err)
			} else {
				require.Error(tt, err)
			}
		})
	}
}
