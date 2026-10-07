package logging

import (
	"bytes"
	"io"
	"testing"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	dslog "github.com/grafana/dskit/log"
)

// realisticAttrs is a message plus ~6 key/values, representative of a real
// fork call site.
var realisticAttrs = []any{
	"alert", "HighErrorRate",
	"receiver", "team-slack",
	"attempt", 3,
	"duration", 1234,
	"peer", "am-0",
	"group_key", "{}:{alertname=\"HighErrorRate\"}",
}

func BenchmarkLogLine_DirectGoKit(b *testing.B) {
	logger := log.NewLogfmtLogger(io.Discard)
	pairs := append([]any{"msg", "notification sent"}, realisticAttrs...)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = level.Info(logger).Log(pairs...)
	}
}

func BenchmarkLogLine_Adapter(b *testing.B) {
	logger := NewSlogLogger(log.NewLogfmtLogger(io.Discard))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.Info("notification sent", realisticAttrs...)
	}
}

// BenchmarkLogLine_AdapterWithCaller is scenario (3) from the Phase B
// go-ahead: adapter with a caller derived from slog.Record.PC. This now
// exercises the real WithCaller() option (option (b)+(d)), not a prototype.
func BenchmarkLogLine_AdapterWithCaller(b *testing.B) {
	logger := NewSlogLogger(log.NewLogfmtLogger(io.Discard), WithCaller())
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.Info("notification sent", realisticAttrs...)
	}
}

// BenchmarkLogLine_AdapterDebugDisabled is scenario (4): adapter with debug
// disabled, logging at debug. Uses a grafana-alertmanager-shaped logger
// (implements DebugEnabled(), unwrapped -- see the package doc comment) so
// Enabled() actually takes the fast path and slog skips building the record
// at all, instead of formatting it and only dropping it at the innermost
// go-kit filter. A logger that doesn't expose DebugEnabled() (e.g.
// grafana/grafana's) does not get this benefit; see BenchmarkLogLine_Adapter
// for the cost of building+dropping instead of skipping.
func BenchmarkLogLine_AdapterDebugDisabled(b *testing.B) {
	var buf bytes.Buffer
	logger := NewSlogLogger(newGrafanaAlertmanagerLogger(&buf, dslog.LogfmtFormat, level.AllowInfo(), false))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.Debug("notification sent", realisticAttrs...)
	}
}
