package v1

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/go-kit/log"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/templates"
)

// mockSNSClient is a mock implementation of snsClient that captures the
// PublishInput passed to Publish.
type mockSNSClient struct {
	publishInput *sns.PublishInput
}

func (m *mockSNSClient) Publish(input *sns.PublishInput) (*sns.PublishOutput, error) {
	m.publishInput = input
	return &sns.PublishOutput{}, nil
}

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
		tmplFn, _ := templates.TmplText(context.Background(), tmpl, alerts, log.NewNopLogger(), &tmplErr)

		snsInput, err := snsNotifier.createPublishInput(context.Background(), tmplFn)
		require.NoError(t, err)
		require.NoError(t, tmplErr)

		require.Equal(t, "AWS SNS", snsNotifier.Name)
		require.Equal(t, schema.SNSType, snsNotifier.Type)
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
			Base: receivers.NewBase(receivers.Metadata{
				Name:                  "AWS SNS",
				Type:                  schema.SNSType,
				UID:                   "",
				DisableResolveMessage: false,
			}, log.NewNopLogger()),
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
		tmplFn, _ := templates.TmplText(context.Background(), tmpl, alerts, log.NewNopLogger(), &tmplErr)

		snsInput, err := snsNotifier.createPublishInput(context.Background(), tmplFn)
		require.NoError(t, err)
		require.NoError(t, tmplErr)

		require.Equal(t, "AWS SNS", snsNotifier.Name)
		require.Equal(t, schema.SNSType, snsNotifier.Type)
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
			Base: receivers.NewBase(receivers.Metadata{
				Name:                  "AWS SNS",
				Type:                  schema.SNSType,
				UID:                   "",
				DisableResolveMessage: false,
			}, log.NewNopLogger()),
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
		tmplFn, _ := templates.TmplText(context.Background(), tmpl, alerts, log.NewNopLogger(), &tmplErr)

		snsInput, err := snsNotifier.createPublishInput(context.Background(), tmplFn)
		require.NoError(t, err)
		require.NoError(t, tmplErr)

		require.Equal(t, "AWS SNS", snsNotifier.Name)
		require.Equal(t, schema.SNSType, snsNotifier.Type)
		require.Equal(t, "abcd", *snsInput.Message)
		require.Equal(t, stringWithManyCharacters[:100], *snsInput.Subject)
		require.Equal(t, "true", *snsInput.MessageAttributes["subject_truncated"].StringValue)
	})
}

func TestNotify_ExtraData(t *testing.T) {
	tmpl := templates.ForTests(t)

	externalURL, err := url.Parse("http://localhost/base")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	settings := Config{
		TopicARN: "arn:aws:sns:us-east-1:123456789:test",
		Message:  `{{ range $i, $a := .Alerts }}Alert {{ $i }}: {{ printf "%s" $a.ExtraData }} {{ end }}`,
	}

	// Create test alerts
	alerts := []*types.Alert{
		{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations: model.LabelSet{"ann1": "annv1"},
			},
		},
		{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert2", "lbl1": "val2"},
				Annotations: model.LabelSet{"ann1": "annv2"},
			},
		},
	}

	// Create extra data that will be passed via context
	extraData1 := json.RawMessage(`{"customField": "customValue1", "priority": "high"}`)
	extraData2 := json.RawMessage(`{"customField": "customValue2", "priority": "medium"}`)
	extraDataSlice := []json.RawMessage{extraData1, extraData2}

	mockClient := &mockSNSClient{}

	snsNotifier := &Notifier{
		Base:     receivers.NewBase(receivers.Metadata{}, log.NewNopLogger()),
		tmpl:     tmpl,
		settings: settings,
		newSNSClient: func(_ func(string) string) (snsClient, error) {
			return mockClient, nil
		},
	}

	// Create context with extra data
	ctx := context.WithValue(context.Background(), receivers.ExtraDataKey, extraDataSlice)

	// Call Notify
	ok, err := snsNotifier.Notify(ctx, alerts...)
	require.NoError(t, err)
	require.True(t, ok)

	// Verify that extra data is present in the published message
	require.NotNil(t, mockClient.publishInput)
	require.Contains(t, *mockClient.publishInput.Message, "customField")
	require.Contains(t, *mockClient.publishInput.Message, "customValue1")
	require.Contains(t, *mockClient.publishInput.Message, "customValue2")
}
