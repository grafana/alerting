package logging

import (
	"bytes"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	dslog "github.com/grafana/dskit/log"
	"github.com/grafana/dskit/spanlogger"
	"github.com/stretchr/testify/require"
)

// consumerStyle builds a pair of identically-decorated go-kit loggers writing
// to separate buffers, matching how a real consumer constructs its logger.
type consumerStyle struct {
	name       string
	newLoggers func() (direct log.Logger, directBuf *bytes.Buffer, viaAdapter log.Logger, adapterBuf *bytes.Buffer)
}

func plainLogfmtLogger() consumerStyle {
	return consumerStyle{
		name: "plain-logfmt",
		newLoggers: func() (log.Logger, *bytes.Buffer, log.Logger, *bytes.Buffer) {
			var directBuf, adapterBuf bytes.Buffer
			return log.NewLogfmtLogger(&directBuf), &directBuf, log.NewLogfmtLogger(&adapterBuf), &adapterBuf
		},
	}
}

// grafanaAlertmanagerLogger replicates grafana-alertmanager's InitLogger
// (pkg/util/log/log.go), non-rate-limited branch: ts + caller baked in at a
// fixed stack depth, then a level filter.
func grafanaAlertmanagerLogger() consumerStyle {
	build := func(buf *bytes.Buffer) log.Logger {
		logger := dslog.NewGoKitWithWriter(dslog.LogfmtFormat, buf)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", spanlogger.Caller(6))
		return level.NewFilter(logger, level.AllowAll())
	}
	return consumerStyle{
		name: "grafana-alertmanager",
		newLoggers: func() (log.Logger, *bytes.Buffer, log.Logger, *bytes.Buffer) {
			var directBuf, adapterBuf bytes.Buffer
			return build(&directBuf), &directBuf, build(&adapterBuf), &adapterBuf
		},
	}
}

// grafanaGrafanaLogger replicates grafana/grafana's ConcreteLogger.Log
// (pkg/infra/log/log.go): every Log call re-wraps the base logger with a "t"
// timestamp valuer; no caller is baked in by default.
func grafanaGrafanaLogger() consumerStyle {
	build := func(buf *bytes.Buffer) log.Logger {
		base := log.NewLogfmtLogger(buf)
		return log.LoggerFunc(func(kv ...any) error {
			return log.With(base, "t", log.TimestampFormat(time.Now, time.RFC3339Nano)).Log(kv...)
		})
	}
	return consumerStyle{
		name: "grafana-grafana",
		newLoggers: func() (log.Logger, *bytes.Buffer, log.Logger, *bytes.Buffer) {
			var directBuf, adapterBuf bytes.Buffer
			return build(&directBuf), &directBuf, build(&adapterBuf), &adapterBuf
		},
	}
}

// parseLogfmt splits a logfmt line into a key->value map for field-by-field
// comparison. Good enough for the fixed key/value shapes these tests emit.
func parseLogfmt(t *testing.T, line string) map[string]string {
	t.Helper()
	fields := map[string]string{}
	for tok := range strings.FieldsSeq(line) {
		kv := strings.SplitN(tok, "=", 2)
		if len(kv) != 2 {
			t.Fatalf("could not parse logfmt token %q in line %q", tok, line)
		}
		fields[kv[0]] = strings.Trim(kv[1], `"`)
	}
	return fields
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestGoldenLines_LevelAndFields logs the same event directly via go-kit and
// via adapter+slog for each consumer decoration style, and compares fields
// other than the known-volatile ones (ts/t exact value, caller: see the
// dedicated caller test below).
func TestGoldenLines_LevelAndFields(t *testing.T) {
	styles := []consumerStyle{plainLogfmtLogger(), grafanaAlertmanagerLogger(), grafanaGrafanaLogger()}

	for _, style := range styles {
		t.Run(style.name, func(t *testing.T) {
			directLogger, directBuf, adapterLogger, adapterBuf := style.newLoggers()

			require.NoError(t, level.Info(directLogger).Log("msg", "hello", "user", "yuri", "count", 3))
			NewSlogLogger(adapterLogger).With("user", "yuri", "count", 3).Info("hello")

			direct := parseLogfmt(t, strings.TrimRight(directBuf.String(), "\n"))
			viaAdapter := parseLogfmt(t, strings.TrimRight(adapterBuf.String(), "\n"))

			volatile := map[string]bool{"ts": true, "t": true, "caller": true}
			for _, k := range sortedKeys(direct) {
				if volatile[k] {
					continue
				}
				require.Equal(t, direct[k], viaAdapter[k], "field %q differs: direct=%q viaAdapter=%q\ndirect line:  %s\nadapter line: %s", k, direct[k], viaAdapter[k], directBuf.String(), adapterBuf.String())
			}
			for _, k := range sortedKeys(viaAdapter) {
				if volatile[k] {
					continue
				}
				_, ok := direct[k]
				require.True(t, ok, "adapter line has extra field %q not present in direct line\ndirect line:  %s\nadapter line: %s", k, directBuf.String(), adapterBuf.String())
			}
		})
	}
}

// TestGoldenLines_DebugFiltering checks that a debug-filtered consumer logger
// still drops Debug records when reached through the adapter.
func TestGoldenLines_DebugFiltering(t *testing.T) {
	var buf bytes.Buffer
	base := dslog.NewGoKitWithWriter(dslog.LogfmtFormat, &buf)
	filtered := level.NewFilter(base, level.AllowInfo())

	NewSlogLogger(filtered).Debug("should not appear")
	require.Empty(t, buf.String())

	NewSlogLogger(filtered).Info("should appear")
	require.Contains(t, buf.String(), "should appear")
}

// TestGoldenLines_WithGroup checks slog group flattening into dotted keys.
func TestGoldenLines_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSlogLogger(log.NewLogfmtLogger(&buf))
	logger.WithGroup("req").With("id", "abc").Info("done")
	fields := parseLogfmt(t, strings.TrimRight(buf.String(), "\n"))
	require.Equal(t, "abc", fields["req.id"])
	require.Equal(t, "done", fields["msg"])
}

// TestGoldenLines_NoDuplicateTimeOrCallerKeys is the specific regression test
// for the two known tjhop/slog-gokit defects called out in the migration
// brief: it must not inject its own "time" key, nor its own "caller" key,
// alongside whatever the underlying go-kit logger already produces.
func TestGoldenLines_NoDuplicateTimeOrCallerKeys(t *testing.T) {
	var buf bytes.Buffer
	base := dslog.NewGoKitWithWriter(dslog.LogfmtFormat, &buf)
	decorated := log.With(base, "ts", log.DefaultTimestampUTC, "caller", spanlogger.Caller(6))

	NewSlogLogger(decorated).Info("hello")
	line := buf.String()

	require.Equal(t, 1, strings.Count(line, "ts="), "line: %s", line)
	require.Equal(t, 1, strings.Count(line, "caller="), "line: %s", line)
	require.NotContains(t, line, "time=", "adapter must not inject its own time key: %s", line)
}

// TestCallerParity is not a pass/fail assertion of parity — it measures
// whether the caller value produced through the adapter matches the one
// produced by a direct go-kit call from the exact same call site, for each
// consumer style. See the package-level finding recorded in the PR/report:
// consumers that bake a fixed-stack-depth caller Valuer into their logger
// before handing it to grafana/alerting (grafana-alertmanager's InitLogger)
// cannot get a correct caller through this adapter, because the extra slog
// stack frames shift the depth the Valuer walks, and grafana/alerting has no
// way to adjust or strip a Valuer already baked into an opaque log.Logger.
func TestCallerParity(t *testing.T) {
	t.Run("grafana-alertmanager style: baked-in spanlogger.Caller(6) does NOT survive the adapter unchanged", func(t *testing.T) {
		var directBuf, adapterBuf bytes.Buffer
		build := func(buf *bytes.Buffer) log.Logger {
			logger := dslog.NewGoKitWithWriter(dslog.LogfmtFormat, buf)
			return log.With(logger, "caller", spanlogger.Caller(6))
		}
		directLogger := build(&directBuf)
		adapterLogger := NewSlogLogger(build(&adapterBuf))

		level.Info(directLogger).Log("msg", "hi") //nolint:errcheck
		adapterLogger.Info("hi")

		directCaller := parseLogfmt(t, strings.TrimRight(directBuf.String(), "\n"))["caller"]
		adapterCaller := parseLogfmt(t, strings.TrimRight(adapterBuf.String(), "\n"))["caller"]

		t.Logf("direct call site caller:  %s", directCaller)
		t.Logf("adapter call site caller: %s", adapterCaller)

		// spanlogger.Caller(N) is a Valuer: it walks the *real* call stack
		// fresh on every Log call, skipping N frames. Measured in this repo
		// (see PR description): a fixed N that reports the correct call site
		// for a direct `level.Info(logger).Log(...)` call needs N=3; reaching
		// the same real call site through this adapter needs N=6 -- slog's
		// Logger.Info -> Logger.log -> Handler.Handle -> our Handle adds
		// exactly 3 stack frames versus the direct path. Both grafana-
		// alertmanager's InitLogger (N=6 or 7) and grafana/alerting's own
		// GrafanaAlertmanagerOpts.Logger use ONE shared logger instance for
		// both direct go-kit call sites (v1 receivers, elsewhere in
		// grafana/alerting) and, after the fork moves to slog, adapter-wrapped
		// calls into the fork. A single fixed N baked into that one instance
		// cannot be correct for both paths at once, and grafana/alerting has
		// no way to strip or re-depth a Valuer already baked into an opaque
		// log.Logger it's handed. This asserts the observed mismatch so a
		// regression (e.g. someone "fixing" the adapter to match by luck)
		// gets caught, not to assert any particular caller value is correct.
		require.NotEqual(t, directCaller, adapterCaller,
			"if this ever becomes equal, re-check: it means the frame-count assumption above no longer holds")
	})
}
