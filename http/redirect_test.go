package http

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSensitiveHeaderStrippingRoundTripper(t *testing.T) {
	tests := []struct {
		name string
		// chain lists the URLs the request has been redirected through, in order. The last entry is
		// the request handed to the round tripper.
		chain       []string
		expStripped bool
	}{
		{
			name:        "not a redirect",
			chain:       []string{"http://example.com/webhook"},
			expStripped: false,
		},
		{
			name:        "redirect on the same host",
			chain:       []string{"http://example.com/webhook", "http://example.com/other"},
			expStripped: false,
		},
		{
			name:        "redirect to a different port on the same host",
			chain:       []string{"http://example.com:8080/webhook", "http://example.com:9090/other"},
			expStripped: false,
		},
		{
			name:        "redirect to a subdomain",
			chain:       []string{"http://example.com/webhook", "http://api.example.com/other"},
			expStripped: false,
		},
		{
			// net/http only compares hostnames, so it keeps sensitive headers on a downgrade to
			// plain HTTP on the same host. Match that.
			name:        "redirect downgrading the scheme on the same host",
			chain:       []string{"https://example.com/webhook", "http://example.com/other"},
			expStripped: false,
		},
		{
			name:        "redirect to a different host",
			chain:       []string{"http://example.com/webhook", "http://evil.com/collect"},
			expStripped: true,
		},
		{
			name:        "redirect from a subdomain to its parent",
			chain:       []string{"http://api.example.com/webhook", "http://example.com/other"},
			expStripped: true,
		},
		{
			name:        "redirect to a host that only looks like a subdomain",
			chain:       []string{"http://example.com/webhook", "http://example.com.evil.com/collect"},
			expStripped: true,
		},
		{
			name: "redirect back to the original host after leaving it",
			chain: []string{
				"http://example.com/webhook",
				"http://evil.com/bounce",
				"http://example.com/other",
			},
			expStripped: true,
		},
		{
			name: "redirect to a different host later in the chain",
			chain: []string{
				"http://example.com/webhook",
				"http://example.com/other",
				"http://evil.com/collect",
			},
			expStripped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := buildRedirectChain(t, tt.chain)
			req.Header.Set("Authorization", "Bearer secret-token")
			req.Header.Set("Www-Authenticate", "Bearer realm=example")
			req.Header.Set("Cookie", "session=secret-session")
			req.Header.Set("Cookie2", "session2=secret-session2")
			req.Header.Set("Proxy-Authorization", "Bearer secret-proxy-token")
			req.Header.Set("Proxy-Authenticate", "Bearer realm=proxy")
			req.Header.Set("X-Custom", "not-sensitive")

			next := &recordingRoundTripper{}
			resp, err := newSensitiveHeaderStrippingRoundTripper(next).RoundTrip(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			require.NotNil(t, next.got, "expected the request to be forwarded")
			if tt.expStripped {
				for _, header := range sensitiveHeaders {
					require.Empty(t, next.got.Header.Get(header),
						"expected the %s header to be stripped", header)
				}
			} else {
				require.Equal(t, "Bearer secret-token", next.got.Header.Get("Authorization"))
				require.Equal(t, "Bearer realm=example", next.got.Header.Get("Www-Authenticate"))
				require.Equal(t, "session=secret-session", next.got.Header.Get("Cookie"))
				require.Equal(t, "session2=secret-session2", next.got.Header.Get("Cookie2"))
				require.Equal(t, "Bearer secret-proxy-token", next.got.Header.Get("Proxy-Authorization"))
				require.Equal(t, "Bearer realm=proxy", next.got.Header.Get("Proxy-Authenticate"))
			}

			require.Equal(t, "not-sensitive", next.got.Header.Get("X-Custom"),
				"headers that are not sensitive should always be forwarded")

			// A RoundTripper must not modify the request it is given.
			require.Equal(t, "Bearer secret-token", req.Header.Get("Authorization"),
				"the caller's request must not be modified")
			require.Equal(t, "session=secret-session", req.Header.Get("Cookie"),
				"the caller's request must not be modified")
		})
	}
}

// TestSensitiveHeaderStrippingRoundTripperPreservesBody covers the request clone the round tripper
// makes to avoid modifying the caller's request: the clone shares the body, so a redirect that
// keeps the body - a 307 or 308 - must still be sent with it.
func TestSensitiveHeaderStrippingRoundTripperPreservesBody(t *testing.T) {
	req := buildRedirectChain(t, []string{"http://example.com/webhook", "http://evil.com/collect"})
	req.Body = io.NopCloser(strings.NewReader("test-body"))
	req.Header.Set("Authorization", "Bearer secret-token")

	next := &recordingRoundTripper{}
	resp, err := newSensitiveHeaderStrippingRoundTripper(next).RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	require.Empty(t, next.got.Header.Get("Authorization"),
		"expected the Authorization header to be stripped")

	body, err := io.ReadAll(next.got.Body)
	require.NoError(t, err)
	require.Equal(t, "test-body", string(body))
}

func TestIsSameOrSubdomain(t *testing.T) {
	tests := []struct {
		host   string
		parent string
		exp    bool
	}{
		{host: "example.com", parent: "example.com", exp: true},
		{host: "sub.example.com", parent: "example.com", exp: true},
		{host: "deep.sub.example.com", parent: "example.com", exp: true},
		{host: "example.com", parent: "sub.example.com", exp: false},
		{host: "notexample.com", parent: "example.com", exp: false},
		{host: "example.com.evil.com", parent: "example.com", exp: false},
		{host: "127.0.0.1", parent: "localhost", exp: false},
		{host: "127.0.0.1", parent: "127.0.0.1", exp: true},
		{host: "::1", parent: "::1", exp: true},
		// A zone must not be mistaken for a subdomain.
		{host: "::1%example.com", parent: "example.com", exp: false},
		{host: "sub.example.com", parent: "", exp: false},
	}

	for _, tt := range tests {
		t.Run(tt.host+" in "+tt.parent, func(t *testing.T) {
			require.Equal(t, tt.exp, isDomainOrSubdomain(tt.host, tt.parent))
		})
	}
}

// buildRedirectChain returns a request for the last of the given URLs, linked back through
// Request.Response the way http.Client links the requests it creates while following redirects.
func buildRedirectChain(t *testing.T, urls []string) *http.Request {
	t.Helper()
	require.NotEmpty(t, urls)

	var req *http.Request
	for _, u := range urls {
		next, err := http.NewRequest(http.MethodPost, u, nil)
		require.NoError(t, err)
		if req != nil {
			next.Response = &http.Response{Request: req}
		}
		req = next
	}

	return req
}

// recordingRoundTripper records the request it was asked to send.
type recordingRoundTripper struct {
	got *http.Request
}

func (rt *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.got = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Request:    req,
	}, nil
}
