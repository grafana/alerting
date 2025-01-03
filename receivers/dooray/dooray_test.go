package dooray

import (
	"context"
	"net/url"
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
			name: "One alert",
			settings: Config{
				Url:         "http://localhost",
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
				IconURL:     "http://localhost/favicon.ico",
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expHeaders: map[string]string{
				"Content-Type": "application/json;charset=UTF-8",
			},
			expMsg:      "{\"botName\":\"[FIRING:1]  (val1)\",\"botIconImage\":\"http://localhost/favicon.ico\",\"text\":\"[FIRING:1]  (val1)\\nhttp:/localhost/alerting/list\\n\\n**Firing**\\n\\nValue: [no value]\\nLabels:\\n - alertname = alert1\\n - lbl1 = val1\\nAnnotations:\\n - ann1 = annv1\\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\\nDashboard: http://localhost/d/abcd\\nPanel: http://localhost/d/abcd?viewPanel=efgh\\n\"}",
			expMsgError: nil,
		}, {
			name: "Multiple alerts",
			settings: Config{
				Url:         "http://localhost",
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
				IconURL:     "http://localhost/favicon.ico",
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
			expHeaders: map[string]string{
				"Content-Type": "application/json;charset=UTF-8",
			},
			expMsg:      "{\"botName\":\"[FIRING:2]  \",\"botIconImage\":\"http://localhost/favicon.ico\",\"text\":\"[FIRING:2]  \\nhttp:/localhost/alerting/list\\n\\n**Firing**\\n\\nValue: [no value]\\nLabels:\\n - alertname = alert1\\n - lbl1 = val1\\nAnnotations:\\n - ann1 = annv1\\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\\n\\nValue: [no value]\\nLabels:\\n - alertname = alert1\\n - lbl1 = val2\\nAnnotations:\\n - ann1 = annv2\\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval2\\n\"}",
			expMsgError: nil,
		}, {
			name: "One alert custom title and description",
			settings: Config{
				Url:         "http://localhost",
				Title:       "customTitle {{ .Alerts.Firing | len }}",
				Description: "customDescription",
				IconURL:     "http://localhost/favicon.ico",
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expHeaders: map[string]string{
				"Content-Type": "application/json;charset=UTF-8",
			},
			expMsg:      "{\"botName\":\"customTitle 1\",\"botIconImage\":\"http://localhost/favicon.ico\",\"text\":\"customTitle 1\\nhttp:/localhost/alerting/list\\n\\ncustomDescription\"}",
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
			require.NoError(t, err)
			require.True(t, ok)

			require.Equal(t, c.expHeaders, webhookSender.Webhook.HTTPHeader)
			require.Equal(t, c.expMsg, webhookSender.Webhook.Body)
		})
	}
}
