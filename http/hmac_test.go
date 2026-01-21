package http

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"
)

func computeHMAC(t *testing.T, secret, body string, clk clock.Clock) string {
	t.Helper()

	hash := hmac.New(sha256.New, []byte(secret))

	if clk != nil {
		ts := strconv.FormatInt(clk.Now().Unix(), 10)
		hash.Write([]byte(ts))
		hash.Write([]byte(":"))
	}
	hash.Write([]byte(body))

	return hex.EncodeToString(hash.Sum(nil))
}

func TestHMACRoundTripper(t *testing.T) {
	mockClock := clock.NewMock()

	testCases := []struct {
		name            string
		secret          string
		header          string
		timestampHeader string
		body            string
		expectErr       bool
	}{
		{
			name:            "Valid signing without timestamp",
			secret:          "secret",
			header:          "X-Signature",
			timestampHeader: "",
			body:            "test message",
		},
		{
			name:            "Valid signing with timestamp",
			secret:          "secret",
			header:          "X-Signature",
			timestampHeader: "X-Timestamp",
			body:            "test message",
		},
		{
			name:            "Empty secret",
			secret:          "",
			header:          "X-Signature",
			timestampHeader: "",
			body:            "test message",
			expectErr:       true,
		},
		{
			name:            "Empty header uses default",
			secret:          "secret",
			header:          "",
			timestampHeader: "",
			body:            "test message",
		},
		{
			name:            "Empty body without timestamp",
			secret:          "secret",
			header:          "X-Signature",
			timestampHeader: "",
			body:            "",
		},
		{
			name:            "Empty body with timestamp",
			secret:          "secret",
			header:          "X-Signature",
			timestampHeader: "X-Timestamp",
			body:            "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rt, err := NewHMACRoundTripper(http.DefaultTransport, mockClock, tc.secret, tc.header, tc.timestampHeader)

			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Create request with body
			body := bytes.NewReader([]byte(tc.body))
			req, err := http.NewRequest(http.MethodPost, "http://example.com", body)
			require.NoError(t, err)

			err = rt.sign(req)
			require.NoError(t, err)

			// Verify the signature and the timestamp
			headerName := tc.header
			if headerName == "" {
				headerName = defaultHeaderName
			}

			var clkForSigning clock.Clock
			if tc.timestampHeader != "" {
				clkForSigning = mockClock
			}
			expectedHash := computeHMAC(t, tc.secret, tc.body, clkForSigning)
			require.Equal(t, expectedHash, req.Header.Get(headerName))

			if tc.timestampHeader != "" {
				ts := strconv.FormatInt(mockClock.Now().Unix(), 10)
				require.Equal(t, ts, req.Header.Get(tc.timestampHeader))
			}

			// Verify that the body can still be read
			if req.Body != nil {
				bodyBytes, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				require.Equal(t, tc.body, string(bodyBytes))
			}
		})
	}
}
