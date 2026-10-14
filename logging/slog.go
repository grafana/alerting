// Package logging bridges go-kit/log loggers to *slog.Logger at the boundary
// where grafana/alerting calls into the (slog-based) prometheus-alertmanager
// fork, without changing the go-kit log lines grafana/grafana and
// grafana-alertmanager emit.
package logging

import (
	"context"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// NewSlogLogger adapts a go-kit log.Logger to a *slog.Logger.
//
// Unlike github.com/tjhop/slog-gokit's GoKitHandler, this adapter injects
// neither a "time" nor a "caller" field of its own: it forwards exactly the
// fields the underlying go-kit logger already produces, so consumers that
// decorate their logger with those fields (grafana-alertmanager,
// grafana/grafana) don't get duplicates or a second, incorrectly-depthed
// caller value.
//
// Enabled(Debug) detection (see Option/WithDebugEnabled) depends on how each
// consumer's logger is built:
//
//   - grafana-alertmanager: GrafanaAlertmanagerOpts.Logger is, unwrapped, the
//     *levelFilter InitLogger returns, which implements DebugEnabled()
//     directly -- calling NewSlogLogger(theOptsLogger) detects it via the
//     type assertion below with no option needed. But go-kit's log.Logger is
//     an opaque interface: wrapping it first with log.With(logger, "component",
//     "x") (the pattern this package's own callers use to scope a sub-logger
//     per fork component, mirroring Mimir's SlogFromGoKit call sites) returns
//     a *log.context, a distinct concrete type that does not implement
//     DebugEnabled() -- Go does not promote it through a non-embedded field.
//     Detection is hidden by that wrapping. Callers that scope a logger this
//     way must probe DebugEnabled() on the *original*, unwrapped logger
//     themselves and pass the result via WithDebugEnabled.
//   - grafana/grafana: its logger is filtered with go-kit's own stock
//     level.NewFilter (pkg/infra/log/log.go), which implements no
//     DebugEnabled()-shaped method at all. Detection never succeeds for this
//     consumer, wrapped or not; Enabled(Debug) always reports true unless a
//     caller supplies WithDebugEnabled explicitly.
//
// In both undetected cases, emitted log output is still correct: the
// underlying go-kit logger's own level filter still drops disallowed levels
// when Log is actually called. Only the Enabled() fast path -- letting a
// caller skip building an expensive Debug record at all -- is unavailable.
func NewSlogLogger(logger log.Logger, opts ...Option) *slog.Logger {
	h := &handler{logger: logger}
	for _, opt := range opts {
		opt(h)
	}
	return slog.New(h)
}

// Option configures NewSlogLogger.
type Option func(*handler)

// WithDebugEnabled explicitly tells the adapter whether Debug-level records
// are enabled, overriding the DebugEnabled() interface probe. Use this when
// handing NewSlogLogger a logger that was itself built with log.With(...) (or
// similar) on top of a logger that implements DebugEnabled() -- that
// wrapping hides the method from Go's interface assertion, so probe it on
// the pre-With logger and pass the result here.
func WithDebugEnabled(enabled bool) Option {
	return func(h *handler) { h.debugEnabled = &enabled }
}

// WithCaller makes the adapter add exactly one "caller" field to every
// record, in the same basename:line format go-kit's own Caller Valuer uses,
// derived from slog.Record.PC -- the call site slog itself captured at the
// real Debug/Info/Warn/Error call, independent of how many extra frames the
// adapter or slog add on top. Unlike a go-kit Valuer's fixed-skip stack walk
// (see the package doc comment's caller-parity discussion), this is exact
// regardless of call depth.
//
// Only pass this when logger does not, and will never, carry its own
// "caller" field: the adapter has no way to detect or remove one already
// baked into an opaque go-kit log.Logger, so combining the two would produce
// a duplicate (logfmt) or silently-overwritten (JSON, which deduplicates
// keys by keeping the last one written) "caller" value.
func WithCaller() Option {
	return func(h *handler) { h.addCaller = true }
}

// handler implements slog.Handler on top of a go-kit log.Logger.
type handler struct {
	logger log.Logger
	// preformatted holds already-flattened key/value pairs from prior
	// WithAttrs calls, ready to append directly to a log.Logger.Log call.
	preformatted []any
	group        string
	debugEnabled *bool
	addCaller    bool
}

func (h *handler) Enabled(_ context.Context, lvl slog.Level) bool {
	// go-kit's level.NewFilter drops disallowed levels when Log is called, so
	// returning true here never produces incorrect output; it only costs a
	// discarded Log call for levels a filter would have dropped anyway.
	if lvl >= slog.LevelInfo {
		return true
	}
	if h.debugEnabled != nil {
		return *h.debugEnabled
	}
	if d, ok := h.logger.(interface{ DebugEnabled() bool }); ok {
		return d.DebugEnabled()
	}
	return true
}

func (h *handler) Handle(_ context.Context, record slog.Record) error {
	pairs := make([]any, 0, 4+len(h.preformatted)+2*record.NumAttrs())
	// h.preformatted (bound via WithAttrs, i.e. logger.With(...)) comes first,
	// matching go-kit's own log.With: "With returns a new contextual logger
	// with keyvals prepended to those passed to calls to Log" -- so a real
	// go-kit call shaped as log.With(l, "k1", v1).Log("msg", m, "k2", v2)
	// emits k1, then msg, then k2, not msg first.
	pairs = append(pairs, h.preformatted...)
	pairs = append(pairs, "msg", record.Message)
	if h.addCaller && record.PC != 0 {
		pairs = append(pairs, "caller", callerFromPC(record.PC))
	}
	record.Attrs(func(a slog.Attr) bool {
		pairs = appendAttr(pairs, h.group, a)
		return true
	})
	return goKitLevel(h.logger, record.Level).Log(pairs...)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	pairs := make([]any, len(h.preformatted), len(h.preformatted)+2*len(attrs))
	copy(pairs, h.preformatted)
	for _, a := range attrs {
		pairs = appendAttr(pairs, h.group, a)
	}
	return &handler{logger: h.logger, preformatted: pairs, group: h.group, debugEnabled: h.debugEnabled, addCaller: h.addCaller}
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	group := name
	if h.group != "" {
		group = h.group + "." + group
	}
	return &handler{logger: h.logger, preformatted: h.preformatted, group: group, debugEnabled: h.debugEnabled, addCaller: h.addCaller}
}

// callerFromPC formats pc as go-kit's own Caller Valuer does: basename:line.
func callerFromPC(pc uintptr) string {
	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	return filepath.Base(frame.File) + ":" + strconv.Itoa(frame.Line)
}

// appendAttr flattens a into pairs, prefixing its key with groupPrefix
// (dotted) and recursing into group-kind attrs the same way slog's own
// handlers do.
func appendAttr(pairs []any, groupPrefix string, a slog.Attr) []any {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return pairs
	}
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		if len(attrs) == 0 {
			return pairs
		}
		if a.Key != "" {
			if groupPrefix != "" {
				groupPrefix += "." + a.Key
			} else {
				groupPrefix = a.Key
			}
		}
		for _, ga := range attrs {
			pairs = appendAttr(pairs, groupPrefix, ga)
		}
		return pairs
	}
	key := a.Key
	if groupPrefix != "" {
		key = groupPrefix + "." + key
	}
	// a.Value.Any() unwraps to the underlying Go value (int, bool, error,
	// time.Time, ...): appending the slog.Value itself would make go-kit's
	// encoders format it via slog.Value.String() instead of the underlying
	// type's own error/Stringer/encoding.TextMarshaler semantics -- e.g. an
	// int 3 would print as the quoted string "3" rather than 3, and a
	// time.Time attr would lose its RFC3339Nano encoding.
	return append(pairs, key, a.Value.Any())
}

func goKitLevel(logger log.Logger, lvl slog.Level) log.Logger {
	switch {
	case lvl >= slog.LevelError:
		return level.Error(logger)
	case lvl >= slog.LevelWarn:
		return level.Warn(logger)
	case lvl >= slog.LevelInfo:
		return level.Info(logger)
	default:
		return level.Debug(logger)
	}
}
