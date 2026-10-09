package logging

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	dslog "github.com/grafana/dskit/log"
	"github.com/stretchr/testify/require"
)

func TestCallerlessLoggerCaller(t *testing.T) {
	var buf bytes.Buffer
	normal := newGrafanaAlertmanagerLogger(&buf, dslog.LogfmtFormat, level.AllowAll(), true)
	callerless := log.With(newPlainLogger(&buf), "ts", log.DefaultTimestampUTC)
	logger := log.With(normal, "component", "alertmanager", "org", 1)
	callerless = log.With(callerless, "component", "alertmanager", "org", 1)

	directLine := nextLine(t)
	require.NoError(t, level.Info(logger).Log("msg", "direct"))
	caller, ok := valueOf(t, decodeLogfmtOrdered(t, buf.String()), "caller")
	require.True(t, ok)
	require.Equal(t, fmt.Sprintf("callerless_test.go:%d", directLine), caller)

	buf.Reset()
	forkLine := nextLine(t)
	NewSlogLogger(logger, WithCallerlessLogger(callerless)).WithGroup("request").With("id", "abc").Info("fork")
	fields := decodeLogfmtOrdered(t, buf.String())
	caller, ok = valueOf(t, fields, "caller")
	require.True(t, ok)
	require.Equal(t, fmt.Sprintf("callerless_test.go:%d", forkLine), caller)
	require.Equal(t, 1, strings.Count(buf.String(), "caller="))
	require.Equal(t, 1, strings.Count(buf.String(), "ts="))
	require.Contains(t, fields, kv{"component", "alertmanager"})
	require.Contains(t, fields, kv{"org", "1"})
	require.Contains(t, fields, kv{"request.id", "abc"})
}

func TestCallerlessLoggerDebugFiltering(t *testing.T) {
	var normalBuf, forkBuf bytes.Buffer
	normal := newGrafanaAlertmanagerLogger(&normalBuf, dslog.LogfmtFormat, level.AllowInfo(), false)
	callerless := level.NewFilter(log.NewLogfmtLogger(&forkBuf), level.AllowInfo())
	logger := NewSlogLogger(normal, WithCallerlessLogger(callerless)).With("component", "fork").WithGroup("request")
	require.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
	logger.Debug("dropped")
	require.Empty(t, normalBuf.String())
	require.Empty(t, forkBuf.String())
	logger.Info("included")
	require.Empty(t, normalBuf.String())
	require.Contains(t, forkBuf.String(), "msg=included")

	require.True(t, NewSlogLogger(normal, WithCallerlessLogger(callerless), WithDebugEnabled(true)).Enabled(context.Background(), slog.LevelDebug))
}
