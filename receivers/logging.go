package receivers

import (
	"log/slog"

	"github.com/go-kit/log"

	"github.com/grafana/alerting/logging"
)

// ForkLogger carries an optional logger for calls into the Alertmanager fork.
// It leaves the notifier's normal go-kit logger unchanged.
type ForkLogger struct {
	slogLogger *slog.Logger
}

// SetSlogLogger supplies the fork logger. Passing nil restores default adaptation.
// Configure it before sharing the notifier or calling Notify.
func (l *ForkLogger) SetSlogLogger(logger *slog.Logger) {
	l.slogLogger = logger
}

// GetSlogLogger returns the supplied fork logger, or adapts the normal logger.
func (l *ForkLogger) GetSlogLogger(fallback log.Logger) *slog.Logger {
	return logging.GetSlogLogger(fallback, l.slogLogger)
}
