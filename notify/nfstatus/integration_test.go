package nfstatus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-kit/log"
	alertingModels "github.com/grafana/alerting/models"
	"github.com/prometheus/alertmanager/notify"
	"github.com/stretchr/testify/mock"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

type fakeNotifier struct {
	retry bool
	err   error
}

func (f *fakeNotifier) Notify(_ context.Context, _ ...*types.Alert) (bool, error) {
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

func (m *mockNotificationHistorian) Record(
	ctx context.Context,
	alerts []*types.Alert,
	retry bool,
	notificationErr error,
	duration time.Duration,
	receiverName string,
	groupLabels model.LabelSet,
	pipelineTime time.Time,
	now time.Time,
) {
	m.Called(ctx, alerts, retry, notificationErr, duration, receiverName, groupLabels, pipelineTime, now)
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

	notificationHistorian.On("Record", mock.Anything, alerts, notifier.retry, notifier.err, mock.Anything, testReceiverName, testGroupLabels, testPipelineTime, mock.Anything).Once()

	_, err := integration.Notify(ctx, alerts...)
	assert.Error(t, err)
	assert.Eventually(t, func() bool {
		return notificationHistorian.AssertExpectations(t)
	}, 1*time.Second, 10*time.Millisecond)
}
