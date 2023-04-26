package email

import (
	"context"
	"net/url"
	"testing"

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

	externalURL, err := url.Parse("http://localhost/base")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	t.Run("with the correct settings it should not fail and produce the expected command", func(t *testing.T) {
		settings := Config{
			SingleEmail: false,
			Addresses: []string{
				"someops@example.com",
				"somedev@example.com",
			},
			Message: "{{ template \"default.title\" . }}",
			Subject: templates.DefaultMessageTitleEmbed,
		}

		emailSender := receivers.MockNotificationService()

		emailNotifier := &Notifier{
			Base: &receivers.Base{
				Name:                  "",
				Type:                  "",
				UID:                   "",
				DisableResolveMessage: false,
			},
			log:      &logging.FakeLogger{},
			ns:       emailSender,
			tmpl:     tmpl,
			settings: settings,
			images:   &images.UnavailableProvider{},
		}

		alerts := []*types.Alert{
			{
				Alert: model.Alert{
					Labels:      model.LabelSet{"alertname": "AlwaysFiring", "severity": "warning"},
					Annotations: model.LabelSet{"runbook_url": "http://fix.me", "__dashboardUid__": "abc", "__panelId__": "5"},
				},
			},
		}

		ok, err := emailNotifier.Notify(context.Background(), alerts...)
		require.NoError(t, err)
		require.True(t, ok)

		expected := map[string]interface{}{
			"subject":      emailSender.EmailSync.Subject,
			"to":           emailSender.EmailSync.To,
			"single_email": emailSender.EmailSync.SingleEmail,
			"template":     emailSender.EmailSync.Template,
			"data":         emailSender.EmailSync.Data,
		}
		require.Equal(t, map[string]interface{}{
			"subject":      "[FIRING:1]  (AlwaysFiring warning)",
			"to":           []string{"someops@example.com", "somedev@example.com"},
			"single_email": false,
			"template":     "ng_alert_notification",
			"data": map[string]interface{}{
				"Title":   "[FIRING:1]  (AlwaysFiring warning)",
				"Message": "[FIRING:1]  (AlwaysFiring warning)",
				"Status":  "firing",
				"Alerts": templates.ExtendedAlerts{
					templates.ExtendedAlert{
						Status:       "firing",
						Labels:       templates.KV{"alertname": "AlwaysFiring", "severity": "warning"},
						Annotations:  templates.KV{"runbook_url": "http://fix.me"},
						Fingerprint:  "15a37193dce72bab",
						SilenceURL:   "http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3DAlwaysFiring&matcher=severity%3Dwarning",
						DashboardURL: "http://localhost/base/d/abc",
						PanelURL:     "http://localhost/base/d/abc?viewPanel=5",
					},
				},
				"GroupLabels":       templates.KV{},
				"CommonLabels":      templates.KV{"alertname": "AlwaysFiring", "severity": "warning"},
				"CommonAnnotations": templates.KV{"runbook_url": "http://fix.me"},
				"ExternalURL":       "http://localhost/base",
				"RuleUrl":           "http://localhost/base/alerting/list",
				"AlertPageUrl":      "http://localhost/base/alerting/list?alertState=firing&view=state",
			},
		}, expected)
	})
}
