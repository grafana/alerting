package threema

import (
	"context"
	"net/url"
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

	images := images2.NewFakeProvider(2)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	cases := []struct {
		name         string
		settings     Config
		alerts       []*types.Alert
		expMsg       string
		expInitError string
		expMsgError  error
	}{
		{
			name: "A single alert with an image",
			settings: Config{
				GatewayID:   "*1234567",
				RecipientID: "87654321",
				APISecret:   "supersecret",
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expMsg:      "from=%2A1234567&secret=supersecret&text=%E2%9A%A0%EF%B8%8F+%5BFIRING%3A1%5D++%28val1%29%0A%0A%2AMessage%3A%2A%0A%2A%2AFiring%2A%2A%0A%0AValue%3A+%5Bno+value%5D%0ALabels%3A%0A+-+alertname+%3D+alert1%0A+-+lbl1+%3D+val1%0AAnnotations%3A%0A+-+ann1+%3D+annv1%0ASilence%3A+http%3A%2F%2Flocalhost%2Falerting%2Fsilence%2Fnew%3Falertmanager%3Dgrafana%26matcher%3Dalertname%253Dalert1%26matcher%3Dlbl1%253Dval1%0ADashboard%3A+http%3A%2F%2Flocalhost%2Fd%2Fabcd%0APanel%3A+http%3A%2F%2Flocalhost%2Fd%2Fabcd%3FviewPanel%3Defgh%0A%0A%2AURL%3A%2A+http%3A%2Flocalhost%2Falerting%2Flist%0A%2AImage%3A%2A+https%3A%2F%2Fwww.example.com%2Ftest-image-1.jpg%0A&to=87654321",
			expMsgError: nil,
		}, {
			name: "Multiple alerts with images",
			settings: Config{
				GatewayID:   "*1234567",
				RecipientID: "87654321",
				APISecret:   "supersecret",
				Title:       templates.DefaultMessageTitleEmbed,
				Description: templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__alertImageToken__": "test-image-1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2", "__alertImageToken__": "test-image-2"},
					},
				},
			},
			expMsg:      "from=%2A1234567&secret=supersecret&text=%E2%9A%A0%EF%B8%8F+%5BFIRING%3A2%5D++%0A%0A%2AMessage%3A%2A%0A%2A%2AFiring%2A%2A%0A%0AValue%3A+%5Bno+value%5D%0ALabels%3A%0A+-+alertname+%3D+alert1%0A+-+lbl1+%3D+val1%0AAnnotations%3A%0A+-+ann1+%3D+annv1%0ASilence%3A+http%3A%2F%2Flocalhost%2Falerting%2Fsilence%2Fnew%3Falertmanager%3Dgrafana%26matcher%3Dalertname%253Dalert1%26matcher%3Dlbl1%253Dval1%0A%0AValue%3A+%5Bno+value%5D%0ALabels%3A%0A+-+alertname+%3D+alert1%0A+-+lbl1+%3D+val2%0AAnnotations%3A%0A+-+ann1+%3D+annv2%0ASilence%3A+http%3A%2F%2Flocalhost%2Falerting%2Fsilence%2Fnew%3Falertmanager%3Dgrafana%26matcher%3Dalertname%253Dalert1%26matcher%3Dlbl1%253Dval2%0A%0A%2AURL%3A%2A+http%3A%2Flocalhost%2Falerting%2Flist%0A%2AImage%3A%2A+https%3A%2F%2Fwww.example.com%2Ftest-image-1.jpg%0A%2AImage%3A%2A+https%3A%2F%2Fwww.example.com%2Ftest-image-2.jpg%0A&to=87654321",
			expMsgError: nil,
		}, {
			name: "A single alert with an image and custom title and description",
			settings: Config{
				GatewayID:   "*1234567",
				RecipientID: "87654321",
				APISecret:   "supersecret",
				Title:       "customTitle {{ .Alerts.Firing | len }}",
				Description: "customDescription",
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expMsg:      "from=%2A1234567&secret=supersecret&text=%E2%9A%A0%EF%B8%8F+customTitle+1%0A%0A%2AMessage%3A%2A%0AcustomDescription%0A%2AURL%3A%2A+http%3A%2Flocalhost%2Falerting%2Flist%0A%2AImage%3A%2A+https%3A%2F%2Fwww.example.com%2Ftest-image-1.jpg%0A&to=87654321",
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
				images:   images,
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

			require.Equal(t, c.expMsg, webhookSender.Webhook.Body)
		})
	}
}
