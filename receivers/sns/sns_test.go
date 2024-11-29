package sns

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestCreatePublishInput(t *testing.T) {
	tmpl := templates.ForTests(t)

	externalURL, err := url.Parse("http://localhost/base")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	t.Run("with templated subject and body", func(t *testing.T) {
		settings := Config{
			TopicARN: "arn:aws:sns:us-east-1:123456789:test",
			Subject:  "{{ len .Alerts.Firing }} subject",
			Message:  "{{ len .Alerts.Firing }} body",
		}

		snsNotifier := &Notifier{
			Base: &receivers.Base{
				Name:                  "AWS SNS",
				Type:                  "sns",
				UID:                   "",
				DisableResolveMessage: false,
			},
			log:      &logging.FakeLogger{},
			tmpl:     tmpl,
			settings: settings,
		}
		alerts := []*types.Alert{
			{
				Alert: model.Alert{
					Labels:      model.LabelSet{"alertname": "AlwaysFiring", "severity": "warning"},
					Annotations: model.LabelSet{"runbook_url": "http://fix.me", "__dashboardUid__": "abc", "__panelId__": "5"},
				},
			},
		}
		var tmplErr error
		data := notify.GetTemplateData(context.Background(), tmpl, alerts, snsNotifier.log)
		tmplFn := notify.TmplText(tmpl, data, &tmplErr)

		snsInput, err := snsNotifier.createPublishInput(context.Background(), tmplFn)
		require.NoError(t, err)
		require.NoError(t, tmplErr)

		require.Equal(t, "AWS SNS", snsNotifier.Name)
		require.Equal(t, "sns", snsNotifier.Type)
		require.Equal(t, "1 body", *snsInput.Message)
		require.Equal(t, "1 subject", *snsInput.Subject)
	})

	t.Run("with truncated message", func(t *testing.T) {
		stringWithManyCharacters := strings.Repeat("abcd", 500)
		settings := Config{
			PhoneNumber: "123-456-7890",
			Message:     stringWithManyCharacters,
		}
		snsNotifier := &Notifier{
			Base: &receivers.Base{
				Name:                  "AWS SNS",
				Type:                  "sns",
				UID:                   "",
				DisableResolveMessage: false,
			},
			log:      &logging.FakeLogger{},
			tmpl:     tmpl,
			settings: settings,
		}
		alerts := []*types.Alert{
			{
				Alert: model.Alert{
					Labels:      model.LabelSet{"alertname": "AlwaysFiring", "severity": "warning"},
					Annotations: model.LabelSet{"runbook_url": "http://fix.me", "__dashboardUid__": "abc", "__panelId__": "5"},
				},
			},
		}

		var tmplErr error
		data := notify.GetTemplateData(context.Background(), tmpl, alerts, snsNotifier.log)
		tmplFn := notify.TmplText(tmpl, data, &tmplErr)

		snsInput, err := snsNotifier.createPublishInput(context.Background(), tmplFn)
		require.NoError(t, err)
		require.NoError(t, tmplErr)

		require.Equal(t, "AWS SNS", snsNotifier.Name)
		require.Equal(t, "sns", snsNotifier.Type)
		require.Equal(t, stringWithManyCharacters[:1600], *snsInput.Message)
		require.Equal(t, "true", *snsInput.MessageAttributes["truncated"].StringValue)
	})

	t.Run("with truncated subject", func(t *testing.T) {
		stringWithManyCharacters := strings.Repeat("abcd", 500)
		settings := Config{
			PhoneNumber: "123-456-7890",
			Message:     "abcd",
			Subject:     stringWithManyCharacters,
		}
		snsNotifier := &Notifier{
			Base: &receivers.Base{
				Name:                  "AWS SNS",
				Type:                  "sns",
				UID:                   "",
				DisableResolveMessage: false,
			},
			log:      &logging.FakeLogger{},
			tmpl:     tmpl,
			settings: settings,
		}
		alerts := []*types.Alert{
			{
				Alert: model.Alert{
					Labels:      model.LabelSet{"alertname": "AlwaysFiring", "severity": "warning"},
					Annotations: model.LabelSet{"runbook_url": "http://fix.me", "__dashboardUid__": "abc", "__panelId__": "5"},
				},
			},
		}

		var tmplErr error
		data := notify.GetTemplateData(context.Background(), tmpl, alerts, snsNotifier.log)
		tmplFn := notify.TmplText(tmpl, data, &tmplErr)

		snsInput, err := snsNotifier.createPublishInput(context.Background(), tmplFn)
		require.NoError(t, err)
		require.NoError(t, tmplErr)

		require.Equal(t, "AWS SNS", snsNotifier.Name)
		require.Equal(t, "sns", snsNotifier.Type)
		require.Equal(t, "abcd", *snsInput.Message)
		require.Equal(t, stringWithManyCharacters[:100], *snsInput.Subject)
		require.Equal(t, "true", *snsInput.MessageAttributes["subject_truncated"].StringValue)
	})
}
