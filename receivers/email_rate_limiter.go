package receivers

import (
	"context"
	"errors"

	"golang.org/x/time/rate"
)

// ErrEmailRateLimited is returned by a rate-limited EmailSender when the
// configured rate limit has been exceeded and a notification is dropped.
var ErrEmailRateLimited = errors.New("email notifications are rate limited")

// NewRateLimitedEmailSender wraps inner so that each SendEmail call consumes
// one token from limiter. When no token is available the call is rejected
// with ErrEmailRateLimited instead of being delayed, matching the existing
// alertmanager rate-limited notifier semantics (no retry, drop the send).
//
// The caller owns limiter and may mutate it at runtime via SetLimit/SetBurst.
// A nil limiter disables rate limiting.
func NewRateLimitedEmailSender(inner EmailSender, limiter *rate.Limiter) EmailSender {
	if limiter == nil {
		return inner
	}
	return &rateLimitedEmailSender{inner: inner, limiter: limiter}
}

type rateLimitedEmailSender struct {
	inner   EmailSender
	limiter *rate.Limiter
}

func (s *rateLimitedEmailSender) SendEmail(ctx context.Context, cmd *SendEmailSettings) error {
	if !s.limiter.Allow() {
		return ErrEmailRateLimited
	}
	return s.inner.SendEmail(ctx, cmd)
}
