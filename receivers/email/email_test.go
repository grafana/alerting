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

	settings := Config{
		SingleEmail: false,
		Addresses: []string{
			"someops@example.com",
			"somedev@example.com",
		},
		Message: "{{ template \"default.title\" . }}",
		Subject: templates.DefaultMessageTitleEmbed,
	}

	t.Run("with the correct settings it should not fail and produce the expected command", func(t *testing.T) {
		emailSender := receivers.MockNotificationService()
		emailNotifier := &Notifier{
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

		expected := receivers.SendEmailSettings{
			Subject:     "[FIRING:1]  (AlwaysFiring warning)",
			To:          []string{"someops@example.com", "somedev@example.com"},
			SingleEmail: false,
			Template:    "ng_alert_notification",
			Data: map[string]interface{}{
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
			EmbeddedContents: []receivers.EmbeddedContent{},
		}

		require.Equal(t, expected, emailSender.EmailSync)
	})

	t.Run("with images", func(t *testing.T) {
		t.Run("provided as URL, should add to Alert data", func(t *testing.T) {
			emailSender := receivers.MockNotificationService()
			provider := images.NewFakeProvider(1)
			emailNotifier := &Notifier{
				log:      &logging.FakeLogger{},
				ns:       emailSender,
				tmpl:     tmpl,
				settings: settings,
				images:   provider,
			}

			alerts := []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "AlwaysFiring", "host": "one"},
						Annotations: model.LabelSet{"__alertImageToken__": "test-image-1"},
					},
				},
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "AlwaysFiring", "host": "two"},
						Annotations: model.LabelSet{"__alertImageToken__": "test-image-1"},
					},
				},
			}

			expected := receivers.SendEmailSettings{
				Subject:     "[FIRING:2]  (AlwaysFiring)",
				To:          []string{"someops@example.com", "somedev@example.com"},
				SingleEmail: false,
				Template:    "ng_alert_notification",
				Data: map[string]interface{}{
					"Title":   "[FIRING:2]  (AlwaysFiring)",
					"Message": "[FIRING:2]  (AlwaysFiring)",
					"Status":  "firing",
					"Alerts": templates.ExtendedAlerts{
						templates.ExtendedAlert{
							Status:      "firing",
							Labels:      templates.KV{"alertname": "AlwaysFiring", "host": "one"},
							Annotations: templates.KV{},
							Fingerprint: "103993fb5f120498",
							SilenceURL:  "http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3DAlwaysFiring&matcher=host%3Done",
							ImageURL:    "https://www.example.com/test-image-1.jpg",
						},
						templates.ExtendedAlert{
							Status:      "firing",
							Labels:      templates.KV{"alertname": "AlwaysFiring", "host": "two"},
							Annotations: templates.KV{},
							Fingerprint: "471d74b314cdd5da",
							SilenceURL:  "http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3DAlwaysFiring&matcher=host%3Dtwo",
							ImageURL:    "https://www.example.com/test-image-1.jpg",
						},
					},
					"GroupLabels":       templates.KV{},
					"CommonLabels":      templates.KV{"alertname": "AlwaysFiring"},
					"CommonAnnotations": templates.KV{},
					"ExternalURL":       "http://localhost/base",
					"RuleUrl":           "http://localhost/base/alerting/list",
					"AlertPageUrl":      "http://localhost/base/alerting/list?alertState=firing&view=state",
				},
				EmbeddedContents: []receivers.EmbeddedContent{},
			}

			ok, err := emailNotifier.Notify(context.Background(), alerts...)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, expected, emailSender.EmailSync)
		})
		t.Run("provided as binary, should add to embedded contents and skip duplicates", func(t *testing.T) {
			emailSender := receivers.MockNotificationService()
			imageStore := images.NewFakeTokenStoreWithFile(t, 1)
			provider := images.NewFakeProviderWithStore(imageStore)
			emailNotifier := &Notifier{
				log:      &logging.FakeLogger{},
				ns:       emailSender,
				tmpl:     tmpl,
				settings: settings,
				images:   provider,
			}

			expectedImage, err := imageStore.Images["test-image-1"].RawData(context.Background())
			require.NoError(t, err)

			alerts := []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "AlwaysFiring", "host": "one"},
						Annotations: model.LabelSet{"__alertImageToken__": "test-image-1"},
					},
				},
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "AlwaysFiring", "host": "two"},
						Annotations: model.LabelSet{"__alertImageToken__": "test-image-1"},
					},
				},
			}

			expected := receivers.SendEmailSettings{
				Subject:     "[FIRING:2]  (AlwaysFiring)",
				To:          []string{"someops@example.com", "somedev@example.com"},
				SingleEmail: false,
				Template:    "ng_alert_notification",
				Data: map[string]interface{}{
					"Title":   "[FIRING:2]  (AlwaysFiring)",
					"Message": "[FIRING:2]  (AlwaysFiring)",
					"Status":  "firing",
					"Alerts": templates.ExtendedAlerts{
						templates.ExtendedAlert{
							Status:        "firing",
							Labels:        templates.KV{"alertname": "AlwaysFiring", "host": "one"},
							Annotations:   templates.KV{},
							Fingerprint:   "103993fb5f120498",
							SilenceURL:    "http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3DAlwaysFiring&matcher=host%3Done",
							EmbeddedImage: "test-image-1.jpg",
						},
						templates.ExtendedAlert{
							Status:        "firing",
							Labels:        templates.KV{"alertname": "AlwaysFiring", "host": "two"},
							Annotations:   templates.KV{},
							Fingerprint:   "471d74b314cdd5da",
							SilenceURL:    "http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3DAlwaysFiring&matcher=host%3Dtwo",
							EmbeddedImage: "test-image-1.jpg",
						},
					},
					"GroupLabels":       templates.KV{},
					"CommonLabels":      templates.KV{"alertname": "AlwaysFiring"},
					"CommonAnnotations": templates.KV{},
					"ExternalURL":       "http://localhost/base",
					"RuleUrl":           "http://localhost/base/alerting/list",
					"AlertPageUrl":      "http://localhost/base/alerting/list?alertState=firing&view=state",
				},
				EmbeddedContents: []receivers.EmbeddedContent{
					{
						Name:    expectedImage.Name,
						Content: expectedImage.Content,
					},
				},
			}

			ok, err := emailNotifier.Notify(context.Background(), alerts...)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, expected, emailSender.EmailSync)
		})
	})
}
