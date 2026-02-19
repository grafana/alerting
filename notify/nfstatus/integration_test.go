package nfstatus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/notify"
	"github.com/stretchr/testify/mock"

	alertingModels "github.com/grafana/alerting/models"
	"github.com/grafana/alerting/receivers"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

type fakeNotifier struct {
	retry bool
	err   error
}

func (f *fakeNotifier) Notify(_ context.Context, _ ...*types.Alert) (bool, error) {
	time.Sleep(10 * time.Millisecond)
	return f.retry, f.err
}

type fakeResolvedSender struct {
	sendResolved bool
}

func (f *fakeResolvedSender) SendResolved() bool {
	return f.sendResolved
}

func TestIntegration(t *testing.T) {
	notifier := &fakeNotifier{}
	rs := &fakeResolvedSender{}
	integration := NewIntegration(notifier, rs, "foo", 42, "bar", nil, log.NewNopLogger())

	// Check wrapped functions work as expected.
	assert.Equal(t, "foo", integration.Name())
	assert.Equal(t, 42, integration.Index())
	rs.sendResolved = false
	assert.Equal(t, false, integration.SendResolved())
	rs.sendResolved = true
	assert.Equal(t, true, integration.SendResolved())

	// Check that status is empty if no notifications have happened.
	lastAttempt, lastDuration, lastError := integration.GetReport()
	assert.Equal(t, time.Time{}, lastAttempt)
	assert.Equal(t, model.Duration(0), lastDuration)
	assert.Equal(t, nil, lastError)

	// Check that status is collected on successful notification.
	notifier.retry = false
	notifier.err = nil
	retry, err := integration.Notify(context.Background())
	assert.Equal(t, notifier.retry, retry)
	assert.NoError(t, notifier.err, err)
	lastAttempt, lastDuration, lastError = integration.GetReport()
	assert.NotEqual(t, time.Time{}, lastAttempt)
	assert.NotEqual(t, model.Duration(0), lastDuration)
	assert.Equal(t, nil, lastError)

	// Check retry is propagated correctly.
	notifier.retry = true
	notifier.err = nil
	retry, err = integration.Notify(context.Background())
	assert.Equal(t, notifier.retry, retry)
	assert.Equal(t, notifier.err, err)

	// Check errors are propagated, and returned in the status.
	notifier.retry = false
	notifier.err = errors.New("An error")
	retry, err = integration.Notify(context.Background())
	assert.Equal(t, notifier.retry, retry)
	assert.Equal(t, notifier.err, err)
	lastAttempt, lastDuration, lastError = integration.GetReport()
	assert.NotEqual(t, time.Time{}, lastAttempt)
	assert.NotEqual(t, model.Duration(0), lastDuration)
	assert.Equal(t, "An error", lastError.Error())
}

type mockNotificationHistorian struct {
	mock.Mock
}

func (m *mockNotificationHistorian) Record(ctx context.Context, nhe NotificationHistoryEntry) {
	m.Called(ctx, nhe)
}

func TestIntegrationWithNotificationHistorian(t *testing.T) {
	notifier := &fakeNotifier{retry: true, err: errors.New("notification error")}
	notificationHistorian := &mockNotificationHistorian{}
	integration := NewIntegration(notifier, &fakeResolvedSender{}, "foo", 42, "bar", notificationHistorian, log.NewNopLogger())
	alerts := []*types.Alert{
		{
			Alert: model.Alert{
				Labels:       model.LabelSet{"alertname": "Alert1", alertingModels.RuleUIDLabel: "testRuleUID"},
				Annotations:  model.LabelSet{"foo": "bar"},
				StartsAt:     time.Now(),
				EndsAt:       time.Now().Add(5 * time.Minute),
				GeneratorURL: "http://localhost/test",
			},
		},
	}

	testReceiverName := "testReceiverName"
	testGroupLabels := model.LabelSet{"key1": "value1"}
	testPipelineTime := time.Date(2025, time.July, 15, 16, 55, 0, 0, time.UTC)
	ctx := notify.WithReceiverName(context.Background(), testReceiverName)
	ctx = notify.WithGroupLabels(ctx, testGroupLabels)
	ctx = notify.WithNow(ctx, testPipelineTime)
	ctx = notify.WithGroupKey(ctx, "testGroupKey")

	// Add extra data.
	ctx = context.WithValue(ctx, receivers.ExtraDataKey, []json.RawMessage{
		json.RawMessage([]byte(`{"foo":"bar"}`)),
	})

	notificationHistorian.On("Record", mock.Anything, mock.Anything).Once()

	_, err := integration.Notify(ctx, alerts...)
	assert.Error(t, err)
	assert.Eventually(t, func() bool {
		// use a separate testing.T instance to avoid failing the main test
		return notificationHistorian.AssertExpectations(&testing.T{})
	}, 1*time.Second, 10*time.Millisecond)

	actual := notificationHistorian.Calls[0].Arguments.Get(1).(NotificationHistoryEntry)
	actual.Duration = 0 // Zero out duration to make comparison easier.
	expected := NotificationHistoryEntry{
		Alerts: []NotificationHistoryAlert{{
			Alert:     alerts[0],
			ExtraData: json.RawMessage([]byte(`{"foo":"bar"}`)),
		}},
		GroupKey:        "testGroupKey",
		Retry:           notifier.retry,
		NotificationErr: notifier.err,
		Duration:        0,
		ReceiverName:    testReceiverName,
		IntegrationName: "foo",
		IntegrationIdx:  42,
		GroupLabels:     testGroupLabels,
		PipelineTime:    testPipelineTime,
	}
	assert.Equal(t, expected, actual)
}

func TestNotificationHistoryEntry_Validate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name              string
		entry             NotificationHistoryEntry
		wantErr           bool
		expectedErrSubstr []string
	}{
		{
			name: "valid entry passes validation",
			entry: NotificationHistoryEntry{
				ReceiverName:    "test-receiver",
				IntegrationName: "test-integration",
				GroupLabels:     model.LabelSet{"foo": "bar"},
				PipelineTime:    now,
				GroupKey:        "test-group-key",
			},
			wantErr: false,
		},
		{
			name: "empty group labels is valid",
			entry: NotificationHistoryEntry{
				ReceiverName:    "test-receiver",
				IntegrationName: "test-integration",
				GroupLabels:     model.LabelSet{},
				PipelineTime:    now,
				GroupKey:        "test-group-key",
			},
			wantErr: false,
		},
		{
			name: "missing receiver name",
			entry: NotificationHistoryEntry{
				ReceiverName:    "",
				IntegrationName: "test-integration",
				GroupLabels:     model.LabelSet{"foo": "bar"},
				PipelineTime:    now,
				GroupKey:        "test-group-key",
			},
			wantErr:           true,
			expectedErrSubstr: []string{"missing receiver name"},
		},
		{
			name: "missing integration name",
			entry: NotificationHistoryEntry{
				ReceiverName:    "test-receiver",
				IntegrationName: "",
				GroupLabels:     model.LabelSet{"foo": "bar"},
				PipelineTime:    now,
				GroupKey:        "test-group-key",
			},
			wantErr:           true,
			expectedErrSubstr: []string{"missing integration name"},
		},
		{
			name: "missing group labels",
			entry: NotificationHistoryEntry{
				ReceiverName:    "test-receiver",
				IntegrationName: "test-integration",
				GroupLabels:     nil,
				PipelineTime:    now,
				GroupKey:        "test-group-key",
			},
			wantErr:           true,
			expectedErrSubstr: []string{"missing group labels"},
		},
		{
			name: "missing pipeline time",
			entry: NotificationHistoryEntry{
				ReceiverName:    "test-receiver",
				IntegrationName: "test-integration",
				GroupLabels:     model.LabelSet{"foo": "bar"},
				PipelineTime:    time.Time{},
				GroupKey:        "test-group-key",
			},
			wantErr:           true,
			expectedErrSubstr: []string{"missing pipeline time"},
		},
		{
			name: "missing group key",
			entry: NotificationHistoryEntry{
				ReceiverName:    "test-receiver",
				IntegrationName: "test-integration",
				GroupLabels:     model.LabelSet{"foo": "bar"},
				PipelineTime:    now,
				GroupKey:        "",
			},
			wantErr:           true,
			expectedErrSubstr: []string{"missing group key"},
		},
		{
			name: "multiple validation errors are joined",
			entry: NotificationHistoryEntry{
				ReceiverName:    "",
				IntegrationName: "",
				GroupLabels:     nil,
				PipelineTime:    time.Time{},
				GroupKey:        "",
			},
			wantErr: true,
			expectedErrSubstr: []string{
				"missing receiver name",
				"missing integration name",
				"missing group labels",
				"missing pipeline time",
				"missing group key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()

			if tt.wantErr {
				assert.Error(t, err)
				for _, substr := range tt.expectedErrSubstr {
					assert.Contains(t, err.Error(), substr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
