package logging

import (
	"log/slog"

	"github.com/go-kit/log"
)

// WithCallerlessLogger uses callerless for adapted records and adds the caller
// from slog.Record.PC. It must have the same context and filtering as the normal
// logger, except that it must not add its own caller field. The normal go-kit
// logger is not wrapped or modified.
func WithCallerlessLogger(callerless log.Logger) Option {
	return func(h *handler) {
		h.logger = callerless
		h.addCaller = true
	}
}

// GetSlogLogger returns supplied when non-nil, otherwise adapting logger with
// the default behavior. This lets APIs accept an optional fork-specific logger.
func GetSlogLogger(logger log.Logger, supplied *slog.Logger) *slog.Logger {
	if supplied != nil {
		return supplied
	}
	return NewSlogLogger(logger)
}
