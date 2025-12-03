package auth

import (
	"encoding/base64"
	"net/http"
)

// BearerTokenRoundTripper injects an Authorization: Bearer <token> header for every request.
// It wraps an existing RoundTripper (typically http.DefaultTransport).
type BearerTokenRoundTripper struct {
	base  http.RoundTripper
	token string
}

// NewBearerTokenRoundTripper returns a new RoundTripper that adds the bearer token.
// If base is nil, http.DefaultTransport is used.
func NewBearerTokenRoundTripper(token string, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &BearerTokenRoundTripper{base: base, token: token}
}

func (b *BearerTokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(req)
}

// BasicAuthRoundTripper injects an Authorization: Basic <base64(user:pass)> header.
type BasicAuthRoundTripper struct {
	base     http.RoundTripper
	username string
	password string
}

// NewBasicAuthRoundTripper returns a new RoundTripper that adds a Basic auth header.
// If base is nil, http.DefaultTransport is used.
func NewBasicAuthRoundTripper(username, password string, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &BasicAuthRoundTripper{base: base, username: username, password: password}
}

func (b *BasicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	creds := b.username + ":" + b.password
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))
	return b.base.RoundTrip(req)
}
