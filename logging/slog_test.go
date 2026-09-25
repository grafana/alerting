package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"runtime"
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

// Log forwards to the embedded Logger through an explicit method instead of
// relying on interface promotion. grafana-alertmanager's real levelFilter
// does the same (it overrides Log to intercept a private probe value) --
// that override adds one real stack frame between any caller and the
// decorated logger's own Log, which matters for the caller-parity tests:
// without it, this fixture would be one frame shallower than production and
// caller depths measured against it wouldn't be faithful.
func (f *gaLevelFilter) Log(keyvals ...any) error {
	return f.Logger.Log(keyvals...)
}

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

// callerFormat matches go-kit's own basename:line caller shape.
var callerFormat = regexp.MustCompile(`^\S+\.go:\d+$`)

// assertTimeAndCallerShape checks presence, count and format of the
// consumer-specific timestamp/caller fields raw decoded fields carry, for
// the given fixture and path ("direct" or "adapter") -- exact timestamp
// values are volatile (wall clock) and caller's exact value is the
// documented parity gap (see TestCallerParity_GrafanaAlertmanagerStyle), so
// this only checks that the expected field(s) are there exactly once, in the
// right format, and that no fixture emits a caller it never bound.
func assertTimeAndCallerShape(t *testing.T, fixtureName, path string, fields []kv) {
	t.Helper()
	switch fixtureName {
	case "plain-logfmt":
		_, hasTS := valueOf(t, fields, "ts")
		_, hasT := valueOf(t, fields, "t")
		_, hasCaller := valueOf(t, fields, "caller")
		require.False(t, hasTS, "%s/%s: plain-logfmt binds no ts", fixtureName, path)
		require.False(t, hasT, "%s/%s: plain-logfmt binds no t", fixtureName, path)
		require.False(t, hasCaller, "%s/%s: plain-logfmt binds no caller", fixtureName, path)
	case "grafana-alertmanager":
		ts, ok := valueOf(t, fields, "ts")
		require.True(t, ok, "%s/%s: grafana-alertmanager always binds ts", fixtureName, path)
		require.Equal(t, 1, countKey(fields, "ts"), "%s/%s: exactly one ts", fixtureName, path)
		_, err := time.Parse(time.RFC3339Nano, ts)
		require.NoError(t, err, "%s/%s: ts %q must be RFC3339Nano (log.DefaultTimestampUTC)", fixtureName, path, ts)

		caller, ok := valueOf(t, fields, "caller")
		require.True(t, ok, "%s/%s: grafana-alertmanager always binds caller", fixtureName, path)
		require.Equal(t, 1, countKey(fields, "caller"), "%s/%s: exactly one caller", fixtureName, path)
		require.Regexp(t, callerFormat, caller, "%s/%s: caller %q must be basename:line", fixtureName, path, caller)
	case "grafana-grafana":
		tVal, ok := valueOf(t, fields, "t")
		require.True(t, ok, "%s/%s: grafana-grafana always binds t", fixtureName, path)
		require.Equal(t, 1, countKey(fields, "t"), "%s/%s: exactly one t", fixtureName, path)
		_, err := time.Parse(time.RFC3339Nano, tVal)
		require.NoError(t, err, "%s/%s: t %q must be RFC3339Nano", fixtureName, path, tVal)

		_, hasCaller := valueOf(t, fields, "caller")
		require.False(t, hasCaller, "%s/%s: grafana-grafana binds no caller (this test doesn't use WithCaller)", fixtureName, path)
	default:
		t.Fatalf("unknown fixture %q", fixtureName)
	}
}

func countKey(fields []kv, key string) int {
	n := 0
	for _, f := range fields {
		if f.key == key {
			n++
		}
	}
	return n
}

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

				rawDirect := decodeLogfmtOrdered(t, directBuf.String())
				rawAdapter := decodeLogfmtOrdered(t, adapterBuf.String())
				assertTimeAndCallerShape(t, fx.name, "direct", rawDirect)
				assertTimeAndCallerShape(t, fx.name, "adapter", rawAdapter)

				// ts/t are volatile (wall clock); caller's exact value is the
				// documented parity gap, covered by the dedicated
				// TestCallerParity_GrafanaAlertmanagerStyle. Presence and
				// format of both are asserted above; strip them before the
				// field-by-field compare of everything else.
				direct := without(rawDirect, "ts", "t", "caller")
				viaAdapter := without(rawAdapter, "ts", "t", "caller")

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

// TestGoldenLines_BoundAttributeOrder_Logfmt is a permanent regression test
// for the field-order bug found in review: Handle() must place
// h.preformatted (bound via slog's own logger.With(...), i.e. WithAttrs)
// before "msg", matching go-kit's own log.With semantics ("With returns a
// new contextual logger with keyvals prepended to those passed to calls to
// Log"). A realistic fork call site binds scope context at construction
// (component/orgID) and per-call context via a further With
// (aggrGroup/group_fingerprint, mirroring dispatch.go's aggrGroup flush log
// line) before finally logging a message plus event-specific attrs
// (alerts). This differs from TestGoldenLines_AllLevelsAndTypedValues_Logfmt
// in using two separate binding layers -- one go-kit-level (component/
// orgID, via the underlying logger itself) and one slog-level (aggrGroup/
// group_fingerprint, via our own WithAttrs) -- because only the slog-level
// one exercises the exact code path (h.preformatted's position in Handle())
// the bug was in. Verified this actually catches the regression: reverting
// Handle() to append "msg" before h.preformatted makes this test fail while
// every other committed test still passes.
func TestGoldenLines_BoundAttributeOrder_Logfmt(t *testing.T) {
	for _, fx := range fixtures() {
		t.Run(fx.name, func(t *testing.T) {
			var directBuf, adapterBuf bytes.Buffer

			directScoped := log.With(fx.build(&directBuf), "component", "alertmanager", "orgID", 1)
			directBound := log.With(directScoped, "aggrGroup", "g1", "group_fingerprint", "fp1")
			require.NoError(t, level.Info(directBound).Log("msg", "flushing", "alerts", 3))

			adapterScoped := log.With(fx.build(&adapterBuf), "component", "alertmanager", "orgID", 1)
			adapterLogger := NewSlogLogger(adapterScoped).With("aggrGroup", "g1", "group_fingerprint", "fp1")
			adapterLogger.Info("flushing", "alerts", 3)

			direct := without(decodeLogfmtOrdered(t, directBuf.String()), "ts", "t", "caller")
			viaAdapter := without(decodeLogfmtOrdered(t, adapterBuf.String()), "ts", "t", "caller")

			// require.Equal on a []kv slice checks order, not just set
			// membership -- that's the point: a transposed msg/aggrGroup
			// pair must fail this, not just a missing/extra field.
			require.Equal(t, direct, viaAdapter,
				"key order must match exactly (not just as a set)\ndirect:  %s\nadapter: %s", directBuf.String(), adapterBuf.String())
		})
	}
}

// TestGoldenLines_BoundMsgOverride_JSON is a permanent regression test for
// the more consequential form of a field-order/duplicate-key bug: a bound
// "msg" combined with the event's own msg at call time. Like
// TestGoldenLines_BoundAttributeOrder_Logfmt, the bound "msg" must be bound
// at the slog level -- via our own WithAttrs (logger.With("msg", ...)), not
// on the underlying go-kit logger -- so it actually lands in h.preformatted
// and exercises Handle()'s ordering of preformatted vs "msg"; binding it on
// the go-kit logger instead would leave h.preformatted empty and pass even
// if Handle() put "msg" first. go-kit's JSON logger dedupes by
// last-write-wins (TestGoldenLines_JSONDeduplicatesKeys), so the event
// message must win over the bound one in both the direct and the adapter
// path -- if Handle() ever placed msg before h.preformatted, the bound
// "msg" would win instead, and JSON's dedup would hide it as a duplicate
// key (there's only ever one "msg" key in the output either way).
func TestGoldenLines_BoundMsgOverride_JSON(t *testing.T) {
	newJSONLogger := func(buf *bytes.Buffer) log.Logger {
		return level.NewFilter(dslog.NewGoKitWithWriter(dslog.JSONFormat, buf), level.AllowAll())
	}

	var directBuf, adapterBuf bytes.Buffer

	directBound := log.With(newJSONLogger(&directBuf), "msg", "bound-msg")
	require.NoError(t, level.Info(directBound).Log("msg", "event-msg"))

	NewSlogLogger(newJSONLogger(&adapterBuf)).With("msg", "bound-msg").Info("event-msg")

	var direct, viaAdapter map[string]any
	require.NoError(t, json.Unmarshal(directBuf.Bytes(), &direct))
	require.NoError(t, json.Unmarshal(adapterBuf.Bytes(), &viaAdapter))

	require.Equal(t, "event-msg", direct["msg"], "reference behavior: go-kit's own last-write-wins JSON dedup")
	require.Equal(t, "event-msg", viaAdapter["msg"], "the adapter must preserve this precedence: the event's own msg wins over a bound one")
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
// nextLine returns the line number of the statement immediately following
// the call to nextLine (runtime.Caller(1) is nextLine's own caller's frame),
// so a test can assert against "the line I'm about to execute" without
// hardcoding a line number that silently goes stale on the next edit.
func nextLine(t *testing.T) int {
	t.Helper()
	_, _, line, ok := runtime.Caller(1)
	require.True(t, ok)
	return line + 1
}

func TestCallerParity_GrafanaAlertmanagerStyle(t *testing.T) {
	var directBuf, adapterBuf bytes.Buffer
	build := func(buf *bytes.Buffer) log.Logger {
		// The real fixture shape: gaLevelFilter's own Log() forwarding frame
		// (matching grafana-alertmanager's real levelFilter override, not
		// bare interface promotion -- see its doc comment), plus consumer
		// context: grafana/alerting's am.logger scopes InitLogger's output
		// with component/tenant via log.With before ever calling it
		// (notify/grafana_alertmanager.go: `log.With(opts.Logger,
		// "component", "alertmanager", opts.TenantKey, opts.TenantID)`).
		base := newGrafanaAlertmanagerLogger(buf, dslog.LogfmtFormat, level.AllowAll(), true)
		return log.With(base, "component", "alertmanager", "org", 1)
	}
	directLogger := build(&directBuf)
	adapterLogger := NewSlogLogger(build(&adapterBuf))

	directCallLine := nextLine(t)
	level.Info(directLogger).Log("msg", "hi") //nolint:errcheck

	adapterCallLine := nextLine(t)
	adapterLogger.Info("hi")

	directCaller, ok := valueOf(t, decodeLogfmtOrdered(t, directBuf.String()), "caller")
	require.True(t, ok)
	adapterCaller, ok := valueOf(t, decodeLogfmtOrdered(t, adapterBuf.String()), "caller")
	require.True(t, ok)

	t.Logf("direct call site caller:  %s (real call site: slog_test.go:%d)", directCaller, directCallLine)
	t.Logf("adapter call site caller: %s (real call site: slog_test.go:%d)", adapterCaller, adapterCallLine)

	// Direct path: with the real fixture shape (levelFilter's Log() frame +
	// consumer-context scoping, matching production), spanlogger.Caller(6)
	// resolves to the actual call site -- this is what correct caller
	// resolution looks like for a direct go-kit call.
	require.Equal(t, fmt.Sprintf("slog_test.go:%d", directCallLine), directCaller,
		"direct path should resolve to the real call site with the production-shaped fixture")

	// Adapter path, same fixture, same N=6: slog's extra
	// Logger.Info -> Logger.log -> Handler.Handle -> our Handle -> logger.Log
	// frames shift the walk, landing inside our own adapter file instead of
	// the real call site -- this IS the caller-parity gap, not a test
	// artifact. Assert what's actually wrong about it (never the real call
	// line) rather than pin a specific stdlib/adapter file:line, which is
	// compiler- and Go-version-sensitive and not the point being tested.
	require.NotEqual(t, fmt.Sprintf("slog_test.go:%d", adapterCallLine), adapterCaller,
		"if this ever becomes equal, re-check: it would mean the frame-count mismatch this test exists to document no longer holds")
	require.NotContains(t, adapterCaller, "slog_test.go",
		"adapter path should not land back in the test file at all with this fixture shape: got %q", adapterCaller)
}

// ---- opt-in caller via WithCaller (option (b)+(d), commander decision 2026-09-25) ----

// TestOptInCaller_GrafanaAlertmanagerStyleMinusCaller: a grafana-alertmanager-
// style logger (ts baked in, as usual) but WITHOUT its own caller field --
// the shape the new optional GrafanaAlertmanagerOpts field is meant to carry
// -- gets an exact, correct caller when WithCaller() is set: the real call
// line, exactly once.
func TestOptInCaller_GrafanaAlertmanagerStyleMinusCaller(t *testing.T) {
	var buf bytes.Buffer
	logger := dslog.NewGoKitWithWriter(dslog.LogfmtFormat, &buf)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC) // no "caller" -- that's the point

	wantLine := nextLine(t)
	NewSlogLogger(logger, WithCaller()).Info("hi")

	line := buf.String()
	require.Equal(t, 1, strings.Count(line, "caller="), "line: %s", line)
	fields := decodeLogfmtOrdered(t, line)
	got, ok := valueOf(t, fields, "caller")
	require.True(t, ok)
	require.Equal(t, fmt.Sprintf("slog_test.go:%d", wantLine), got)
}

// TestOptInCaller_GrafanaGrafanaStyleUnset: the option unset (grafana/grafana's
// fallback -- no optional caller-less logger configured) must stay exactly
// as it is today: no caller key at all, byte-identical to before this option
// existed.
func TestOptInCaller_GrafanaGrafanaStyleUnset(t *testing.T) {
	var buf bytes.Buffer
	NewSlogLogger(newGrafanaGrafanaLogger(&buf, level.AllowAll())).Info("hi")
	require.NotContains(t, buf.String(), "caller=")
}

// TestOptInCaller_NeverDoubledWithExistingCaller documents (it is the
// caller's responsibility, per WithCaller's doc comment, not something the
// adapter can detect) what happens if WithCaller is combined with a logger
// that already carries its own "caller" field: a visible duplicate in
// logfmt. This is exactly why the optional GrafanaAlertmanagerOpts field is
// documented as "a logger WITHOUT a caller field" -- WithCaller must never be
// paired with the plain Logger field, which may carry one.
func TestOptInCaller_NeverDoubledWithExistingCaller(t *testing.T) {
	var buf bytes.Buffer
	logger := log.With(dslog.NewGoKitWithWriter(dslog.LogfmtFormat, &buf), "caller", "pre-existing:1")
	NewSlogLogger(logger, WithCaller()).Info("hi")
	require.Equal(t, 2, strings.Count(buf.String(), "caller="), "misuse produces a visible duplicate, as documented: %s", buf.String())
}
