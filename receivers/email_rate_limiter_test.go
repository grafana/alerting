package receivers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

type fakeEmailSender struct {
	calls int
	err   error
}

func (f *fakeEmailSender) SendEmail(_ context.Context, _ *SendEmailSettings) error {
	f.calls++
	return f.err
}

func TestNewRateLimitedEmailSender_NilLimiterReturnsInner(t *testing.T) {
	inner := &fakeEmailSender{}
	got := NewRateLimitedEmailSender(inner, nil)
	require.Same(t, inner, got)
}

func TestRateLimitedEmailSender_BlocksAllWhenRateZero(t *testing.T) {
	inner := &fakeEmailSender{}
	s := NewRateLimitedEmailSender(inner, rate.NewLimiter(0, 0))

	err := s.SendEmail(context.Background(), &SendEmailSettings{})
	require.ErrorIs(t, err, ErrEmailRateLimited)
	require.Equal(t, 0, inner.calls)
}

func TestRateLimitedEmailSender_PassesThroughWhenUnlimited(t *testing.T) {
	inner := &fakeEmailSender{}
	s := NewRateLimitedEmailSender(inner, rate.NewLimiter(rate.Inf, 0))

	for range 5 {
		require.NoError(t, s.SendEmail(context.Background(), &SendEmailSettings{}))
	}
	require.Equal(t, 5, inner.calls)
}

func TestRateLimitedEmailSender_EnforcesBurst(t *testing.T) {
	inner := &fakeEmailSender{}
	// 1 token/sec, burst=2: first two calls succeed, third is rejected.
	s := NewRateLimitedEmailSender(inner, rate.NewLimiter(1, 2))

	require.NoError(t, s.SendEmail(context.Background(), &SendEmailSettings{}))
	require.NoError(t, s.SendEmail(context.Background(), &SendEmailSettings{}))
	require.ErrorIs(t, s.SendEmail(context.Background(), &SendEmailSettings{}), ErrEmailRateLimited)
	require.Equal(t, 2, inner.calls)
}

func TestRateLimitedEmailSender_PropagatesInnerError(t *testing.T) {
	sentinel := errors.New("boom")
	inner := &fakeEmailSender{err: sentinel}
	s := NewRateLimitedEmailSender(inner, rate.NewLimiter(rate.Inf, 0))

	err := s.SendEmail(context.Background(), &SendEmailSettings{})
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, 1, inner.calls)
}
