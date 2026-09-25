package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/go-logfmt/logfmt"
	dslog "github.com/grafana/dskit/log"
	"github.com/grafana/dskit/spanlogger"
	"github.com/stretchr/testify/require"
)

// ---- consumer fixtures, built the way each real consumer builds its logger ----

// gaLevelFilter replicates grafana-alertmanager's own pkg/util/log.levelFilter
// (unexported there): a log.Logger that also implements DebugEnabled(), which
// is what lets our adapter's Enabled() detect the configured level directly
// on the *unwrapped* logger InitLogger returns.
type gaLevelFilter struct {
	log.Logger
	debug bool
}

func (f *gaLevelFilter) DebugEnabled() bool { return f.debug }

// newGrafanaAlertmanagerLogger mirrors grafana-alertmanager's InitLogger
// (pkg/util/log/log.go), non-rate-limited branch, field for field: ts +
// caller baked in via spanlogger.Caller(6), then a level filter.
func newGrafanaAlertmanagerLogger(buf *bytes.Buffer, format string, allow level.Option, debugEnabled bool) log.Logger {
	logger := dslog.NewGoKitWithWriter(format, buf)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", spanlogger.Caller(6))
	return &gaLevelFilter{Logger: level.NewFilter(logger, allow), debug: debugEnabled}
}

// newGrafanaGrafanaLogger mirrors grafana/grafana's ConcreteLogger.Log
// (pkg/infra/log/log.go): every Log call re-wraps the base logger with a "t"
// timestamp valuer, filtered with go-kit's own stock level.NewFilter --
// which, unlike grafana-alertmanager's levelFilter, adds no DebugEnabled()
// method at all.
func newGrafanaGrafanaLogger(buf *bytes.Buffer, allow level.Option) log.Logger {
	base := log.NewLogfmtLogger(buf)
	dynamic := log.LoggerFunc(func(kv ...any) error {
		return log.With(base, "t", log.TimestampFormat(time.Now, time.RFC3339Nano)).Log(kv...)
	})
	return level.NewFilter(dynamic, allow)
}

func newPlainLogger(buf *bytes.Buffer) log.Logger {
	return level.NewFilter(log.NewLogfmtLogger(buf), level.AllowAll())
}

type fixture struct {
	name       string
	build      func(buf *bytes.Buffer) log.Logger // AllowAll, debug-aware where the real consumer is
	debugAware bool                               // whether DebugEnabled() is reachable on the unwrapped logger
}

func fixtures() []fixture {
	return []fixture{
		{"plain-logfmt", newPlainLogger, false},
		{"grafana-alertmanager", func(buf *bytes.Buffer) log.Logger {
			return newGrafanaAlertmanagerLogger(buf, dslog.LogfmtFormat, level.AllowAll(), true)
		}, true},
		{"grafana-grafana", func(buf *bytes.Buffer) log.Logger {
			return newGrafanaGrafanaLogger(buf, level.AllowAll())
		}, false},
	}
}

// ---- ordered logfmt decoding (map-based comparison loses order/duplicates) ----

type kv struct{ key, value string }

func decodeLogfmtOrdered(t *testing.T, line string) []kv {
	t.Helper()
	dec := logfmt.NewDecoder(strings.NewReader(line))
	var out []kv
	for dec.ScanRecord() {
		for dec.ScanKeyval() {
			out = append(out, kv{string(dec.Key()), string(dec.Value())})
		}
	}
	require.NoError(t, dec.Err(), "line: %s", line)
	return out
}

func valueOf(t *testing.T, fields []kv, key string) (string, bool) {
	t.Helper()
	for _, f := range fields {
		if f.key == key {
			return f.value, true
		}
	}
	return "", false
}

func without(fields []kv, keys ...string) []kv {
	drop := map[string]bool{}
	for _, k := range keys {
		drop[k] = true
	}
	out := make([]kv, 0, len(fields))
	for _, f := range fields {
		if !drop[f.key] {
			out = append(out, f)
		}
	}
	return out
}

// ---- level dispatch helpers, so a table can drive both APIs identically ----

func logDirect(logger log.Logger, lvl slog.Level, msg string, kvs ...any) {
	pairs := append([]any{"msg", msg}, kvs...)
	var err error
	switch {
	case lvl >= slog.LevelError:
		err = level.Error(logger).Log(pairs...)
	case lvl >= slog.LevelWarn:
		err = level.Warn(logger).Log(pairs...)
	case lvl >= slog.LevelInfo:
		err = level.Info(logger).Log(pairs...)
	default:
		err = level.Debug(logger).Log(pairs...)
	}
	if err != nil {
		panic(err)
	}
}

func logAdapter(logger log.Logger, lvl slog.Level, msg string, kvs ...any) {
	l := NewSlogLogger(logger)
	switch {
	case lvl >= slog.LevelError:
		l.Error(msg, kvs...)
	case lvl >= slog.LevelWarn:
		l.Warn(msg, kvs...)
	case lvl >= slog.LevelInfo:
		l.Info(msg, kvs...)
	default:
		l.Debug(msg, kvs...)
	}
}

var allLevels = []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}

// ---- typed-value parity: every level, logfmt ----

type plainStruct struct{ A, B int }

type customStringer struct{ n int }

func (c customStringer) String() string { return fmt.Sprintf("custom(%d)", c.n) }

func TestGoldenLines_AllLevelsAndTypedValues_Logfmt(t *testing.T) {
	fixedTime := time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC)
	testErr := errors.New("boom")
	attrs := []any{
		"n", 3,
		"ok", true,
		"missing", nil,
		"at", fixedTime,
		"err", testErr,
		"obj", plainStruct{A: 1, B: 2},
		"str", customStringer{5},
	}

	for _, fx := range fixtures() {
		for _, lvl := range allLevels {
			t.Run(fx.name+"/"+lvl.String(), func(t *testing.T) {
				var directBuf, adapterBuf bytes.Buffer
				logDirect(fx.build(&directBuf), lvl, "hello", attrs...)
				logAdapter(fx.build(&adapterBuf), lvl, "hello", attrs...)

				direct := decodeLogfmtOrdered(t, directBuf.String())
				viaAdapter := decodeLogfmtOrdered(t, adapterBuf.String())

				// ts/t/caller are covered by dedicated tests (they're either
				// volatile -- wall clock -- or, for caller, the documented
				// parity gap); strip them before the field-by-field compare.
				direct = without(direct, "ts", "t", "caller")
				viaAdapter = without(viaAdapter, "ts", "t", "caller")

				require.Equal(t, direct, viaAdapter, "direct line:  %s\nadapter line: %s", directBuf.String(), adapterBuf.String())
			})
		}
	}
}

// TestGoldenLines_LogValuer documents that slog.LogValuer resolution has no
// direct-go-kit equivalent to compare against: a LogValuer is resolved by
// slog itself, before our Handle ever sees the attr, so logging the same raw
// struct straight through go-kit (bypassing slog entirely) would just format
// it with %v, not call LogValue(). This asserts the adapter's own behavior
// instead of a parity delta.
type lazyValue struct{ v string }

func (l lazyValue) LogValue() slog.Value { return slog.StringValue("resolved:" + l.v) }

func TestGoldenLines_LogValuer(t *testing.T) {
	var buf bytes.Buffer
	NewSlogLogger(newPlainLogger(&buf)).Info("hello", "lazy", lazyValue{"x"})
	fields := decodeLogfmtOrdered(t, buf.String())
	got, ok := valueOf(t, fields, "lazy")
	require.True(t, ok)
	require.Equal(t, "resolved:x", got)
}

// ---- typed-value parity: JSON, type-aware (not string-coerced) ----

func TestGoldenLines_TypedValues_JSON(t *testing.T) {
	fixedTime := time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC)
	testErr := errors.New("boom")
	attrs := []any{
		"n", 3,
		"ok", true,
		"missing", nil,
		"at", fixedTime,
		"err", testErr,
		"obj", plainStruct{A: 1, B: 2},
	}

	newJSONLogger := func(buf *bytes.Buffer) log.Logger {
		return level.NewFilter(dslog.NewGoKitWithWriter(dslog.JSONFormat, buf), level.AllowAll())
	}

	var directBuf, adapterBuf bytes.Buffer
	logDirect(newJSONLogger(&directBuf), slog.LevelInfo, "hello", attrs...)
	logAdapter(newJSONLogger(&adapterBuf), slog.LevelInfo, "hello", attrs...)

	var direct, viaAdapter map[string]any
	require.NoError(t, json.Unmarshal(directBuf.Bytes(), &direct))
	require.NoError(t, json.Unmarshal(adapterBuf.Bytes(), &viaAdapter))

	// n: a JSON number, not a quoted string.
	require.Equal(t, float64(3), direct["n"])
	require.Equal(t, float64(3), viaAdapter["n"])
	// ok: a JSON bool, not a quoted string.
	require.Equal(t, true, direct["ok"])
	require.Equal(t, true, viaAdapter["ok"])
	// missing: JSON null, not the string "<nil>".
	require.Nil(t, direct["missing"])
	require.Nil(t, viaAdapter["missing"])
	require.Contains(t, directBuf.String(), `"missing":null`)
	require.Contains(t, adapterBuf.String(), `"missing":null`)
	// at: time.Time's own MarshalJSON (RFC3339Nano), identical both paths.
	require.Equal(t, direct["at"], viaAdapter["at"])
	require.Equal(t, fixedTime.Format(`"`+time.RFC3339Nano+`"`), mustCompactJSON(t, direct["at"]))
	// err: error's message string, identical both paths.
	require.Equal(t, testErr.Error(), direct["err"])
	require.Equal(t, testErr.Error(), viaAdapter["err"])
	// obj: a real nested JSON object, not a %v-formatted string.
	require.Equal(t, map[string]any{"A": float64(1), "B": float64(2)}, direct["obj"])
	require.Equal(t, map[string]any{"A": float64(1), "B": float64(2)}, viaAdapter["obj"])
}

func mustCompactJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

// TestGoldenLines_JSONDeduplicatesKeys documents (verified, not assumed) that
// go-kit's JSON logger builds a map[string]interface{} from its keyvals, so a
// duplicate key collapses to the last-written value -- unlike its logfmt
// logger, which writes every occurrence. This corrects an earlier report
// claim that neither format deduplicates.
func TestGoldenLines_JSONDeduplicatesKeys(t *testing.T) {
	var buf bytes.Buffer
	logger := dslog.NewGoKitWithWriter(dslog.JSONFormat, &buf)
	require.NoError(t, logger.Log("caller", "first", "caller", "second"))
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	require.Equal(t, "second", decoded["caller"], "JSON logger should keep only the last occurrence of a duplicate key")

	buf.Reset()
	logfmtLogger := log.NewLogfmtLogger(&buf)
	require.NoError(t, logfmtLogger.Log("caller", "first", "caller", "second"))
	require.Equal(t, 2, strings.Count(buf.String(), "caller="), "logfmt logger keeps every occurrence of a duplicate key")
}

// ---- debug filtering / Enabled() ----

func TestEnabled_DetectionPerConsumer(t *testing.T) {
	t.Run("grafana-alertmanager: DebugEnabled reachable on the unwrapped logger", func(t *testing.T) {
		var buf bytes.Buffer
		infoOnly := newGrafanaAlertmanagerLogger(&buf, dslog.LogfmtFormat, level.AllowInfo(), false)
		l := NewSlogLogger(infoOnly)
		require.False(t, l.Enabled(t.Context(), slog.LevelDebug), "info-filtered logger should report Debug disabled")
		require.True(t, l.Enabled(t.Context(), slog.LevelInfo))

		debugAllowed := newGrafanaAlertmanagerLogger(&buf, dslog.LogfmtFormat, level.AllowDebug(), true)
		require.True(t, NewSlogLogger(debugAllowed).Enabled(t.Context(), slog.LevelDebug))
	})

	t.Run("grafana-alertmanager: wrapping with log.With hides DebugEnabled -- Enabled(Debug) reports true regardless", func(t *testing.T) {
		var buf bytes.Buffer
		infoOnly := newGrafanaAlertmanagerLogger(&buf, dslog.LogfmtFormat, level.AllowInfo(), false)
		scoped := log.With(infoOnly, "component", "inhibitor")
		l := NewSlogLogger(scoped)
		require.True(t, l.Enabled(t.Context(), slog.LevelDebug), "detection is hidden by log.With, so it falls back to the always-true default")

		// The actually-written output is still correct even though Enabled()
		// couldn't predict it: go-kit's own filter drops the Debug line.
		NewSlogLogger(scoped).Debug("should be dropped")
		require.Empty(t, buf.String())
	})

	t.Run("grafana-alertmanager: WithDebugEnabled recovers detection through log.With", func(t *testing.T) {
		var buf bytes.Buffer
		infoOnly := newGrafanaAlertmanagerLogger(&buf, dslog.LogfmtFormat, level.AllowInfo(), false)
		scoped := log.With(infoOnly, "component", "inhibitor")
		l := NewSlogLogger(scoped, WithDebugEnabled(false))
		require.False(t, l.Enabled(t.Context(), slog.LevelDebug))
	})

	t.Run("grafana-grafana: no DebugEnabled-shaped method exists at all", func(t *testing.T) {
		var buf bytes.Buffer
		infoOnly := newGrafanaGrafanaLogger(&buf, level.AllowInfo())
		l := NewSlogLogger(infoOnly)
		require.True(t, l.Enabled(t.Context(), slog.LevelDebug), "no detection is possible for this consumer; always-true default applies")

		// Output is still correct: go-kit's stock filter drops the line.
		l.Debug("should be dropped")
		require.Empty(t, buf.String())
	})
}

func TestDebugFiltering_EveryThreshold(t *testing.T) {
	thresholds := []struct {
		name      string
		allow     level.Option
		wantDebug bool
		wantInfo  bool
		wantWarn  bool
		wantError bool
	}{
		{"AllowDebug", level.AllowDebug(), true, true, true, true},
		{"AllowInfo", level.AllowInfo(), false, true, true, true},
		{"AllowWarn", level.AllowWarn(), false, false, true, true},
		{"AllowError", level.AllowError(), false, false, false, true},
	}
	for _, fx := range fixtures() {
		for _, th := range thresholds {
			t.Run(fx.name+"/"+th.name, func(t *testing.T) {
				var buf bytes.Buffer
				logger := fx.build(&buf)
				// re-derive the fixture at this threshold instead of AllowAll
				switch fx.name {
				case "grafana-alertmanager":
					logger = newGrafanaAlertmanagerLogger(&buf, dslog.LogfmtFormat, th.allow, th.wantDebug)
				case "grafana-grafana":
					logger = newGrafanaGrafanaLogger(&buf, th.allow)
				case "plain-logfmt":
					logger = level.NewFilter(log.NewLogfmtLogger(&buf), th.allow)
				}
				l := NewSlogLogger(logger)

				l.Debug("d")
				gotDebug := strings.Contains(buf.String(), "msg=d")
				buf.Reset()
				l.Info("i")
				gotInfo := strings.Contains(buf.String(), "msg=i")
				buf.Reset()
				l.Warn("w")
				gotWarn := strings.Contains(buf.String(), "msg=w")
				buf.Reset()
				l.Error("e")
				gotError := strings.Contains(buf.String(), "msg=e")

				require.Equal(t, th.wantDebug, gotDebug, "debug")
				require.Equal(t, th.wantInfo, gotInfo, "info")
				require.Equal(t, th.wantWarn, gotWarn, "warn")
				require.Equal(t, th.wantError, gotError, "error")
			})
		}
	}
}

// ---- WithAttrs / WithGroup: nested groups, inline (empty-key) groups, independent branches ----

func TestGoldenLines_GroupsAndIndependentBranches(t *testing.T) {
	var buf bytes.Buffer
	root := NewSlogLogger(newPlainLogger(&buf))

	// Nested group.
	root.WithGroup("req").With("id", "abc").WithGroup("user").With("name", "yuri").Info("nested")
	fields := decodeLogfmtOrdered(t, buf.String())
	got, ok := valueOf(t, fields, "req.id")
	require.True(t, ok)
	require.Equal(t, "abc", got)
	got, ok = valueOf(t, fields, "req.user.name")
	require.True(t, ok)
	require.Equal(t, "yuri", got)

	// Inline group: a slog.Group with an empty key flattens its attrs into
	// the current prefix instead of adding another level.
	buf.Reset()
	root.Info("inline", slog.Group("", slog.String("a", "1"), slog.String("b", "2")))
	fields = decodeLogfmtOrdered(t, buf.String())
	got, ok = valueOf(t, fields, "a")
	require.True(t, ok)
	require.Equal(t, "1", got)
	got, ok = valueOf(t, fields, "b")
	require.True(t, ok)
	require.Equal(t, "2", got)

	// Independent branches from the same parent must not see each other's attrs.
	buf.Reset()
	base := NewSlogLogger(newPlainLogger(&buf))
	branchA := base.With("branch", "a")
	branchB := base.With("branch", "b")
	branchA.Info("from-a")
	fromA := decodeLogfmtOrdered(t, buf.String())
	buf.Reset()
	branchB.Info("from-b")
	fromB := decodeLogfmtOrdered(t, buf.String())

	branchVal, ok := valueOf(t, fromA, "branch")
	require.True(t, ok)
	require.Equal(t, "a", branchVal)
	branchVal, ok = valueOf(t, fromB, "branch")
	require.True(t, ok)
	require.Equal(t, "b", branchVal)
}

// ---- the two known tjhop/slog-gokit defects this adapter must not have ----

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

// ---- caller parity ----
//
// This does not assert mere inequality between the direct and adapter paths
// (that would pass even if both were wrong in different ways). It pins the
// actual, observed values from a real run of this exact test, so a change
// that happens to make them equal -- for the wrong reason -- fails loudly
// instead of silently passing a weaker check.
func TestCallerParity_GrafanaAlertmanagerStyle(t *testing.T) {
	var directBuf, adapterBuf bytes.Buffer
	build := func(buf *bytes.Buffer) log.Logger {
		l := dslog.NewGoKitWithWriter(dslog.LogfmtFormat, buf)
		return log.With(l, "caller", spanlogger.Caller(6))
	}
	directLogger := build(&directBuf)
	adapterLogger := NewSlogLogger(build(&adapterBuf))

	level.Info(directLogger).Log("msg", "hi") //nolint:errcheck
	adapterLogger.Info("hi")

	directCaller, ok := valueOf(t, decodeLogfmtOrdered(t, directBuf.String()), "caller")
	require.True(t, ok)
	adapterCaller, ok := valueOf(t, decodeLogfmtOrdered(t, adapterBuf.String()), "caller")
	require.True(t, ok)

	t.Logf("direct call site caller:  %s", directCaller)
	t.Logf("adapter call site caller: %s", adapterCaller)

	// With spanlogger.Caller(6) -- grafana-alertmanager's real, production
	// depth -- called directly from this test function's own body (no extra
	// t.Run wrapper frame), the direct path runs off the end of the real call
	// stack entirely (6 is tuned for a deeper production call chain than
	// this flat test body has), while slog's extra Logger.Info ->
	// Logger.log -> Handler.Handle -> our Handle -> logger.Log frames happen,
	// in this exact call shape, to add up to just enough extra depth that
	// the adapter path lands back on the real call site. That's a
	// coincidence of this test's specific nesting, not a property of the
	// adapter: a differently-nested caller (see slog-caller-report.md and
	// the reviewer's own reproduction, which used one more layer of nesting
	// and got direct=<the real line>, adapter=inside logging/slog.go
	// instead) gets a different wrong answer. The invariant that holds
	// everywhere is that one fixed N cannot be correct for both call shapes
	// at once; which side is wrong, and how, is call-shape-dependent -- this
	// test pins what THIS exact call shape produces so a change that makes
	// the two paths equal (for the wrong reason) still fails loudly.
	require.Equal(t, "<unknown>", directCaller, "direct path: spanlogger.Caller(6) should run off the stack in this flat call shape")
	// 468 is the line of `adapterLogger.Info("hi")` above -- pinned deliberately
	// (see comment above): if this line moves, update the constant with it.
	require.Equal(t, "slog_test.go:468", adapterCaller,
		"adapter path: spanlogger.Caller(6) happens to land back on the real call site in this call shape -- a coincidence of nesting depth, not a guarantee")
}
