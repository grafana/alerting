package utils

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSlogFromGoKit(t *testing.T) {
	// Basic test that adapter works
	mLogger := &mockLogger{}
	mLogger.On("Log", mock.Anything).Return(nil)

	slogger := SlogFromGoKit(mLogger)

	// All levels should be enabled
	assert.True(t, slogger.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, slogger.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, slogger.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, slogger.Enabled(context.Background(), slog.LevelError))
}

type mockLogger struct {
	mock.Mock
}

func (m *mockLogger) Log(keyvals ...interface{}) error {
	args := m.Called(keyvals)
	return args.Error(0)
}
