package webex

import (
	"context"
	"fmt"
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
		name         string
		settings     Config
		alerts       []*types.Alert
		expHeaders   map[string]string
		expMsg       string
		expInitError string
		expMsgError  error
	}{
		{
			name: "A single alert with default template",
			settings: Config{
				Message: templates.DefaultMessageEmbed,
				RoomID:  "someid",
				APIURL:  DefaultAPIURL,
				Token:   "abcdefgh0123456789",
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
			expHeaders:  map[string]string{"Authorization": "Bearer abcdefgh0123456789"},
			expMsg:      `{"roomId":"someid","markdown":"**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: a URL\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana\u0026matcher=alertname%3Dalert1\u0026matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n","files":["https://www.example.com/test-image-1"]}`,
			expMsgError: nil,
		},
		{
			name: "Multiple alerts with custom template",
			settings: Config{
				Message: "__Custom Firing__\n{{len .Alerts.Firing}} Firing\n{{ template \"__text_alert_list\" .Alerts.Firing }}",
				RoomID:  "someid",
				APIURL:  DefaultAPIURL,
				Token:   "abcdefgh0123456789",
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
			expHeaders:  map[string]string{"Authorization": "Bearer abcdefgh0123456789"},
			expMsg:      `{"roomId":"someid","markdown":"__Custom Firing__\n2 Firing\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: a URL\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana\u0026matcher=alertname%3Dalert1\u0026matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana\u0026matcher=alertname%3Dalert1\u0026matcher=lbl1%3Dval2\n","files":["https://www.example.com/test-image-1"]}`,
			expMsgError: nil,
		},
		{
			name: "Truncate long message",
			settings: Config{
				Message: "{{ .CommonLabels.alertname }}",
				RoomID:  "someid",
				APIURL:  DefaultAPIURL,
				Token:   "abcdefgh0123456789",
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels: model.LabelSet{"alertname": model.LabelValue(strings.Repeat("1", 4097))},
					},
				},
			},
			expHeaders:  map[string]string{"Authorization": "Bearer abcdefgh0123456789"},
			expMsg:      fmt.Sprintf(`{"roomId":"someid","markdown":"%s…"}`, strings.Repeat("1", 4093)),
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

			require.NoError(t, err)
			require.Equal(t, c.expHeaders, notificationService.Webhook.HTTPHeader)
			require.JSONEq(t, c.expMsg, notificationService.Webhook.Body)
		})
	}
}
