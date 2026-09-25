// Package logging bridges go-kit/log loggers to *slog.Logger at the boundary
// where grafana/alerting calls into the (slog-based) prometheus-alertmanager
// fork, without changing the go-kit log lines grafana/grafana and
// grafana-alertmanager emit.
package logging

import (
	"context"
	"log/slog"

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
// caller value. See CallerParity in the package doc comment for the caveat
// this implies.
func NewSlogLogger(logger log.Logger) *slog.Logger {
	return slog.New(&handler{logger: logger})
}

// handler implements slog.Handler on top of a go-kit log.Logger.
type handler struct {
	logger log.Logger
	// preformatted holds already-flattened key/value pairs from prior
	// WithAttrs calls, ready to append directly to a log.Logger.Log call.
	preformatted []any
	group        string
}

func (h *handler) Enabled(_ context.Context, lvl slog.Level) bool {
	// go-kit's level.NewFilter drops disallowed levels when Log is called, so
	// returning true here never produces incorrect output; it only costs a
	// discarded Log call for levels a filter would have dropped anyway. Loggers
	// that expose DebugEnabled (grafana-alertmanager's levelFilter) let us skip
	// that call for the common debug case.
	if lvl >= slog.LevelInfo {
		return true
	}
	if d, ok := h.logger.(interface{ DebugEnabled() bool }); ok {
		return d.DebugEnabled()
	}
	return true
}

func (h *handler) Handle(_ context.Context, record slog.Record) error {
	pairs := make([]any, 0, 2+len(h.preformatted)+2*record.NumAttrs())
	pairs = append(pairs, "msg", record.Message)
	pairs = append(pairs, h.preformatted...)
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
	return &handler{logger: h.logger, preformatted: pairs, group: h.group}
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	group := name
	if h.group != "" {
		group = h.group + "." + group
	}
	return &handler{logger: h.logger, preformatted: h.preformatted, group: group}
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
	return append(pairs, key, a.Value)
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
