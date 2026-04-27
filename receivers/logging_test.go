package receivers

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers/schema"
)

func TestBase_LogNotificationSent(t *testing.T) {
	var buf bytes.Buffer
	base := NewBase(Metadata{
		Name:    "my-receiver",
		Type:    schema.IntegrationType("slack"),
		Version: schema.Version("v1"),
		Index:   0,
	}, log.NewLogfmtLogger(&buf))

	base.LogNotificationSent(context.Background(), 5)

	out := buf.String()
	require.Contains(t, out, `level=info`)
	require.Contains(t, out, `msg="Notification sent"`)
	require.Contains(t, out, `alerts=5`)
	require.Contains(t, out, `receiver=my-receiver`)
	require.Contains(t, out, `integration=slack[0]`)
	require.Contains(t, out, `version=v1`)
}

func TestBase_LogNotificationFailed(t *testing.T) {
	var buf bytes.Buffer
	base := NewBase(Metadata{
		Name:    "my-receiver",
		Type:    schema.IntegrationType("webhook"),
		Version: schema.Version("v1"),
		Index:   1,
	}, log.NewLogfmtLogger(&buf))

	base.LogNotificationFailed(context.Background(), 3, errors.New("connection refused"))

	out := buf.String()
	require.Contains(t, out, `level=error`)
	require.Contains(t, out, `msg="Failed to send notification"`)
	require.Contains(t, out, `alerts=3`)
	require.Contains(t, out, `err="connection refused"`)
	require.Contains(t, out, `receiver=my-receiver`)
	require.Contains(t, out, `integration=webhook[1]`)
}

func TestBase_LogNotificationFailed_WithExtraContext(t *testing.T) {
	var buf bytes.Buffer
	base := NewBase(Metadata{
		Name:    "my-receiver",
		Type:    schema.IntegrationType("discord"),
		Version: schema.Version("v1"),
	}, log.NewLogfmtLogger(&buf))

	base.LogNotificationFailed(context.Background(), 1,
		errors.New("upstream rejected"),
		LogKeyStatusCode, 503,
		"body", "service unavailable",
	)

	out := buf.String()
	require.Contains(t, out, `msg="Failed to send notification"`)
	require.Contains(t, out, `err="upstream rejected"`)
	require.Contains(t, out, `status_code=503`)
	require.Contains(t, out, `body="service unavailable"`)
}

func TestLogNotificationSent_FreeFunc(t *testing.T) {
	var buf bytes.Buffer
	logger := log.NewLogfmtLogger(&buf)

	LogNotificationSent(logger, 7, "messageId", "abc123")

	out := buf.String()
	require.Contains(t, out, `level=info`)
	require.Contains(t, out, `msg="Notification sent"`)
	require.Contains(t, out, `alerts=7`)
	require.Contains(t, out, `messageId=abc123`)
}

func TestLogNotificationFailed_FreeFunc(t *testing.T) {
	var buf bytes.Buffer
	logger := log.NewLogfmtLogger(&buf)

	LogNotificationFailed(logger, 2, errors.New("network down"))

	out := buf.String()
	require.Contains(t, out, `level=error`)
	require.Contains(t, out, `msg="Failed to send notification"`)
	require.Contains(t, out, `alerts=2`)
	require.Contains(t, out, `err="network down"`)
}
