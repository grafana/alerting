package nfstatus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

type fakeNotifier struct {
	retry bool
	err   error
}

func (f *fakeNotifier) Notify(ctx context.Context, alerts ...*types.Alert) (bool, error) {
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
	integration := NewIntegration(notifier, rs, "foo", 42, "bar")

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
