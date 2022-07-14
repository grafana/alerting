package channels

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/alerting"
)

func TestTelegramNotifier(t *testing.T) {
	tmpl, err := template.FromGlobs("templates/default.tmpl")
	require.NoError(t, err)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	cases := []struct {
		name         string
		settings     string
		alerts       []*types.Alert
		expMsg       map[string]string
		expInitError error
		expMsgError  error
	}{
		{
			name: "Default template with one alert",
			settings: `{
				"bottoken": "abcdefgh0123456789",
				"chatid": "someid"
			}`,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1"},
						GeneratorURL: "a URL",
					},
				},
			},
			expMsg: map[string]string{
				"chat_id":    "someid",
				"parse_mode": "html",
				"text":       "\n**Firing**\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: a URL\n\n\n\n\n",
			},
			expInitError: nil,
			expMsgError:  nil,
		}, {
			name: "Custom template with multiple alerts",
			settings: `{
				"bottoken": "abcdefgh0123456789",
				"chatid": "someid",
				"message": "__Custom Firing__\n{{len .Alerts.Firing}} Firing\n{{ template \"__text_alert_list\" .Alerts.Firing }}"
			}`,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1"},
						GeneratorURL: "a URL",
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2"},
					},
				},
			},
			expMsg: map[string]string{
				"chat_id":    "someid",
				"parse_mode": "html",
				"text":       "__Custom Firing__\n2 Firing\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: a URL\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSource: \n",
			},
			expInitError: nil,
			expMsgError:  nil,
		}, {
			name:         "Error in initing",
			settings:     `{}`,
			expInitError: alerting.ValidationError{Reason: "Could not find Bot Token in settings"},
		}, {
			name: "Error in building message",
			settings: `{
				"bottoken": "abcdefgh0123456789",
				"chatid": "someid",
				"message": "{{ .BrokenTemplate }"
			}`,
			expMsgError: errors.New("template: :1: unexpected \"}\" in operand"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			settingsJSON, err := simplejson.NewJson([]byte(c.settings))
			require.NoError(t, err)

			m := &models.AlertNotification{
				Name:     "telegram_testing",
				Type:     "telegram",
				Settings: settingsJSON,
			}

			pn, err := NewTelegramNotifier(m, tmpl)
			if c.expInitError != nil {
				require.Error(t, err)
				require.Equal(t, c.expInitError.Error(), err.Error())
				return
			}
			require.NoError(t, err)

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
			msg, err := pn.buildTelegramMessage(ctx, c.alerts)
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
