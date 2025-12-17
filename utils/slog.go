package utils

import (
	"log/slog"

	"github.com/go-kit/log"
	slgk "github.com/tjhop/slog-gokit"
)

// SlogFromGoKit returns slog adapter for a go-kit logger.
// All log levels are enabled (no level filtering).
func SlogFromGoKit(logger log.Logger) *slog.Logger {
	lvl := slog.LevelVar{}
	lvl.Set(slog.LevelDebug) // Enable all levels
	return slog.New(slgk.NewGoKitHandler(logger, &lvl))
}
