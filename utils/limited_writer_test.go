package utils

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimitedWriter(t *testing.T) {
	tests := []struct {
		name          string
		limit         int64
		writes        [][]byte
		expectedData  string
		expectedError error
		errorOnWrite  int // which write should return an error (0-indexed)
	}{
		{
			name:         "single write under limit",
			limit:        100,
			writes:       [][]byte{[]byte("hello")},
			expectedData: "hello",
		},
		{
			name:         "single write exactly at limit",
			limit:        5,
			writes:       [][]byte{[]byte("hello")},
			expectedData: "hello",
		},
		{
			name:          "single write exceeds limit",
			limit:         5,
			writes:        [][]byte{[]byte("hello world")},
			expectedData:  "hello",
			expectedError: ErrWriteLimitExceeded,
			errorOnWrite:  1,
		},
		{
			name:         "multiple writes under limit",
			limit:        20,
			writes:       [][]byte{[]byte("hello"), []byte(" "), []byte("world")},
			expectedData: "hello world",
		},
		{
			name:         "multiple writes exactly at limit",
			limit:        11,
			writes:       [][]byte{[]byte("hello"), []byte(" "), []byte("world")},
			expectedData: "hello world",
		},
		{
			name:          "multiple writes exceed limit on second write",
			limit:         10,
			writes:        [][]byte{[]byte("hello"), []byte(" world")},
			expectedData:  "hello worl",
			expectedError: ErrWriteLimitExceeded,
			errorOnWrite:  2,
		},
		{
			name:          "multiple writes exceed limit on third write",
			limit:         8,
			writes:        [][]byte{[]byte("hello"), []byte(" wo"), []byte("rld")},
			expectedData:  "hello wo",
			expectedError: ErrWriteLimitExceeded,
			errorOnWrite:  3,
		},
		{
			name:          "write after hitting limit",
			limit:         5,
			writes:        [][]byte{[]byte("hello"), []byte("world")},
			expectedData:  "hello",
			expectedError: ErrWriteLimitExceeded,
			errorOnWrite:  2,
		},
		{
			name:          "write after exceeding limit",
			limit:         3,
			writes:        [][]byte{[]byte("hello"), []byte("world")},
			expectedData:  "hel",
			expectedError: ErrWriteLimitExceeded,
			errorOnWrite:  1,
		},
		{
			name:         "zero byte writes",
			limit:        10,
			writes:       [][]byte{[]byte("hello"), []byte(""), []byte("world")},
			expectedData: "helloworld",
		},
		{
			name:         "empty writes only",
			limit:        10,
			writes:       [][]byte{[]byte(""), []byte(""), []byte("")},
			expectedData: "",
		},
		{
			name:          "write to zero limit",
			limit:         0,
			writes:        [][]byte{[]byte("hello")},
			expectedData:  "",
			expectedError: ErrWriteLimitExceeded,
			errorOnWrite:  1,
		},
		{
			name:          "many small writes exceed limit",
			limit:         10,
			writes:        [][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("d"), []byte("e"), []byte("f"), []byte("g"), []byte("h"), []byte("i"), []byte("j"), []byte("k")},
			expectedData:  "abcdefghij",
			expectedError: ErrWriteLimitExceeded,
			errorOnWrite:  11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			lw := NewLimitedWriter(buf, tt.limit)

			var err error
			var errIndex int
			for i, data := range tt.writes {
				_, err = lw.Write(data)
				if err != nil {
					errIndex = i
					break
				}
			}
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError, "expected error %v, got %v", tt.expectedError, err)
				assert.Equal(t, tt.errorOnWrite, errIndex+1, "expected error on write %d, got %d", tt.errorOnWrite, errIndex)
			} else {
				require.NoErrorf(t, err, "expected no error, got %v on write %d", err, errIndex)
			}
			assert.Equal(t, tt.expectedData, buf.String(), "written data mismatch")
		})
	}
}

func TestLimitedWriter_UnderlyingWriterError(t *testing.T) {
	expectedErr := errors.New("underlying writer error")

	// Create a writer that always fails
	failWriter := &failingWriter{err: expectedErr}
	lw := NewLimitedWriter(failWriter, 100)

	n, err := lw.Write([]byte("hello"))

	// Should get the underlying writer's error, not the limit error
	require.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 0, n)
}

func TestLimitedWriter_PartialWriteFromUnderlyingWriter(t *testing.T) {
	// Create a writer that only writes 3 bytes at a time
	buf := &bytes.Buffer{}
	partialWriter := &partialWriter{w: buf, maxBytes: 3}
	lw := NewLimitedWriter(partialWriter, 100)

	n, err := lw.Write([]byte("hello"))

	// Should write only 3 bytes and return no error
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, "hel", buf.String())
}

// failingWriter is a writer that always returns an error
type failingWriter struct {
	err error
}

func (w *failingWriter) Write(p []byte) (n int, err error) {
	return 0, w.err
}

// partialWriter is a writer that only writes a limited number of bytes per call
type partialWriter struct {
	w        io.Writer
	maxBytes int
}

func (w *partialWriter) Write(p []byte) (n int, err error) {
	if len(p) > w.maxBytes {
		p = p[:w.maxBytes]
	}
	return w.w.Write(p)
}
