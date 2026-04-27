package receivers

import (
	"context"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// Standard log keys, event messages, and helpers used by notification integrations.
//
// All integrations log via go-kit/log. Base.GetLogger automatically decorates
// every line with: receiver, integration, version, aggrGroup. Notification
// code paths should use the constants below for additional fields and event
// messages so log aggregation tools (e.g. Loki) can filter and correlate
// across integrations with a single query.

// Standard log field keys.
//
// Auto-injected by Base.GetLogger and not redefined here:
//
//	receiver, integration, version, aggrGroup.
const (
	// LogKeyActualLen is the actual length of content that was truncated.
	LogKeyActualLen = "actualLen"

	// LogKeyAlerts is the number of alerts in the notification batch.
	LogKeyAlerts = "alerts"

	// LogKeyErr is the error returned by a failed operation.
	LogKeyErr = "err"

	// LogKeyField names the config or template field a log refers to. Used by
	// template-rendering errors and content-truncation warnings.
	LogKeyField = "field"

	// LogKeyMaxLen is the maximum allowed length for a truncated field.
	LogKeyMaxLen = "maxLen"

	// LogKeyRecipient is the notification destination identifier (URL, channel,
	// topic, email address, issue key, ...). The value varies per integration;
	// the key is fixed.
	LogKeyRecipient = "recipient"

	// LogKeyStatusCode is the HTTP status code returned by the destination.
	// Use the integer status code, not the textual "200 OK" form.
	LogKeyStatusCode = "status_code"
)

// Standard event messages. Use these as the static "msg" value so operators
// can grep for an event across every integration with a single query.
const (
	LogMsgContentTruncated         = "Content truncated"
	LogMsgFailedToSendNotification = "Failed to send notification"
	LogMsgNotificationSent         = "Notification sent"
	LogMsgSendingNotification      = "Sending notification"
	LogMsgTemplateRenderingFailed  = "Template rendering failed"
)

// LogNotificationSent logs a successful notification dispatch at INFO level
// using the provided logger directly. Pass integration-specific context as
// keyvals (e.g. "messageId", id). Use this from notifiers that do not embed
// *Base; from notifiers that do, prefer (*Base).LogNotificationSent.
func LogNotificationSent(logger log.Logger, alertCount int, keyvals ...interface{}) {
	args := append([]interface{}{
		"msg", LogMsgNotificationSent,
		LogKeyAlerts, alertCount,
	}, keyvals...)
	level.Info(logger).Log(args...)
}

// LogNotificationFailed logs a failed notification dispatch at ERROR level
// using the provided logger directly. Pass integration-specific context as
// keyvals (e.g. "body", body, LogKeyStatusCode, code). Use this from
// notifiers that do not embed *Base; from notifiers that do, prefer
// (*Base).LogNotificationFailed.
func LogNotificationFailed(logger log.Logger, alertCount int, err error, keyvals ...interface{}) {
	args := append([]interface{}{
		"msg", LogMsgFailedToSendNotification,
		LogKeyAlerts, alertCount,
		LogKeyErr, err,
	}, keyvals...)
	level.Error(logger).Log(args...)
}

// LogNotificationSent logs a successful notification dispatch at INFO level.
// Call this at the success exit of Notify so operators can confirm delivery
// in log aggregation tools rather than inferring it from the absence of an
// error. Pass integration-specific context as keyvals.
func (n *Base) LogNotificationSent(ctx context.Context, alertCount int, keyvals ...interface{}) {
	LogNotificationSent(n.GetLogger(ctx), alertCount, keyvals...)
}

// LogNotificationFailed logs a failed notification dispatch at ERROR level.
// Call this at the failure exit of Notify so operators can find failures
// across every integration with a single message-grep. Pass
// integration-specific context as keyvals.
func (n *Base) LogNotificationFailed(ctx context.Context, alertCount int, err error, keyvals ...interface{}) {
	LogNotificationFailed(n.GetLogger(ctx), alertCount, err, keyvals...)
}
