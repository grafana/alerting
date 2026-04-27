package receivers

import (
	"context"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// Event messages and helpers used by notification integrations.
//
// All integrations log via go-kit/log. Base.GetLogger automatically decorates
// every line with: receiver, integration, version, aggrGroup. Notification
// dispatch code should call the helpers below so log aggregation tools (e.g.
// Loki) can filter and correlate across integrations with a single query.

// Canonical event messages. Use these as the static "msg" value so operators
// can grep for an event across every integration with a single query.
const (
	LogMsgFailedToSendNotification = "Failed to send notification"
	LogMsgNotificationSent         = "Notification sent"
)

// LogOption attaches an optional structured field to a notification log line.
// Construct options via the With* functions in this package.
type LogOption func(*[]any)

// WithStatusCode adds the HTTP status code returned by the destination.
func WithStatusCode(code int) LogOption {
	return func(kv *[]any) {
		*kv = append(*kv, "status_code", code)
	}
}

// WithRequestBody adds the request body sent to the destination.
func WithRequestBody(body string) LogOption {
	return func(kv *[]any) {
		*kv = append(*kv, "request_body", body)
	}
}

// WithResponseBody adds the response body returned by the destination.
func WithResponseBody(body string) LogOption {
	return func(kv *[]any) {
		*kv = append(*kv, "response_body", body)
	}
}

// LogNotificationSent logs a successful notification dispatch at DEBUG level
// using the provided logger directly. Use this from notifiers that do not
// embed *Base; from notifiers that do, prefer (*Base).LogNotificationSent.
func LogNotificationSent(logger log.Logger, alertCount int, opts ...LogOption) {
	kv := []any{
		"msg", LogMsgNotificationSent,
		"alerts", alertCount,
	}
	for _, opt := range opts {
		opt(&kv)
	}
	level.Debug(logger).Log(kv...)
}

// LogNotificationFailed logs a failed notification dispatch at WARN level
// using the provided logger directly. Use this from notifiers that do not
// embed *Base; from notifiers that do, prefer (*Base).LogNotificationFailed.
func LogNotificationFailed(logger log.Logger, alertCount int, err error, opts ...LogOption) {
	kv := []any{
		"msg", LogMsgFailedToSendNotification,
		"alerts", alertCount,
		"err", err,
	}
	for _, opt := range opts {
		opt(&kv)
	}
	level.Warn(logger).Log(kv...)
}

// LogNotificationSent logs a successful notification dispatch at DEBUG level.
// Call this at the success exit of Notify so operators can confirm delivery
// in log aggregation tools rather than inferring it from the absence of an
// error.
func (n *Base) LogNotificationSent(ctx context.Context, alertCount int, opts ...LogOption) {
	LogNotificationSent(n.GetLogger(ctx), alertCount, opts...)
}

// LogNotificationFailed logs a failed notification dispatch at WARN level.
// Call this at the failure exit of Notify so operators can find failures
// across every integration with a single message-grep.
func (n *Base) LogNotificationFailed(ctx context.Context, alertCount int, err error, opts ...LogOption) {
	LogNotificationFailed(n.GetLogger(ctx), alertCount, err, opts...)
}
