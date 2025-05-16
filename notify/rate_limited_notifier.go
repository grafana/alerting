// SPDX-License-Identifier: AGPL-3.0-only
// Provenance-includes-location: https://github.com/cortexproject/cortex/blob/master/pkg/alertmanager/rate_limited_notifier.go
// Provenance-includes-license: Apache-2.0
// Provenance-includes-copyright: The Cortex Authors.

package notify

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
	"golang.org/x/time/rate"
)

type DefaultRateLimitsProvider interface {
	Get(integrationType string) *RateLimits
}

type RateLimiterFactory func(upstream notify.Notifier, cfg NotifierConfigBase) notify.Notifier

func NewRateLimiterWrapperFactory(recheckInterval time.Duration, rateLimitedNotifications *prometheus.CounterVec, provider DefaultRateLimitsProvider) WrapNotifierFunc {
	return func(cfg NotifierConfigBase, upstream notify.Notifier) notify.Notifier {
		limits := cfg.RateLimits
		if limits == nil && provider != nil {
			limits = provider.Get(cfg.Type)
		}
		if limits == nil {
			return upstream
		}
		return newRateLimitedNotifier(upstream, *limits, recheckInterval, rateLimitedNotifications.WithLabelValues(cfg.Type))
	}
}

type rateLimitedNotifier struct {
	upstream notify.Notifier
	counter  prometheus.Counter

	limiter *rate.Limiter
	limits  RateLimits

	recheckInterval time.Duration
	recheckAt       atomic.Int64 // unix nanoseconds timestamp
}

func newRateLimitedNotifier(upstream notify.Notifier, limits RateLimits, recheckInterval time.Duration, counter prometheus.Counter) *rateLimitedNotifier {
	return &rateLimitedNotifier{
		upstream:        upstream,
		counter:         counter,
		limits:          limits,
		limiter:         rate.NewLimiter(limits.RateLimit, limits.Burst),
		recheckInterval: recheckInterval,
	}
}

var ErrRateLimited = errors.New("failed to notify due to rate limits")

func (r *rateLimitedNotifier) Notify(ctx context.Context, alerts ...*types.Alert) (bool, error) {
	now := time.Now()
	if now.UnixNano() >= r.recheckAt.Load() {
		if limit := r.limits.RateLimit; r.limiter.Limit() != limit {
			r.limiter.SetLimitAt(now, limit)
		}

		if burst := r.limits.Burst; r.limiter.Burst() != burst {
			r.limiter.SetBurstAt(now, burst)
		}

		r.recheckAt.Store(now.UnixNano() + r.recheckInterval.Nanoseconds())
	}

	// This counts as single notification, no matter how many alerts there are in it.
	if !r.limiter.AllowN(now, 1) {
		r.counter.Inc()
		// Don't retry this notification later.
		return false, ErrRateLimited
	}

	return r.upstream.Notify(ctx, alerts...)
}
