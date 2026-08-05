package http

import (
	"net/http"
	"strings"
)

// sensitiveHeaders are the headers that must not follow a request to another host. This is the same
// set that net/http itself refuses to copy across a cross-host redirect, see
// shouldCopyHeaderOnRedirect in net/http/client.go.
var sensitiveHeaders = []string{
	"Authorization",
	"Www-Authenticate",
	"Cookie",
	"Cookie2",
}

// SensitiveHeaderStrippingRoundTripper removes sensitive headers from requests that http.Client
// created by following a redirect to a different host.
//
// http.Client already refuses to copy those headers across a cross-host redirect, but it can only
// do so for the headers that were set on the request before it was handed to the transport. Round
// trippers that add credentials themselves - OAuth2RoundTripper and HMACRoundTripper - run after
// that, and add them to every hop of a redirect chain, so without this the credentials are sent to
// whatever host the request is redirected to.
//
// Wrap the transport that talks to the network with this, before wrapping it with anything that
// adds credentials, so that it runs last and sees the request as it is about to be sent.
type SensitiveHeaderStrippingRoundTripper struct {
	next http.RoundTripper
}

func NewSensitiveHeaderStrippingRoundTripper(next http.RoundTripper) *SensitiveHeaderStrippingRoundTripper {
	return &SensitiveHeaderStrippingRoundTripper{next: next}
}

func (rt *SensitiveHeaderStrippingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	next := rt.next
	if next == nil {
		next = http.DefaultTransport
	}

	if !isCrossHostRedirect(req) {
		return next.RoundTrip(req)
	}

	// A RoundTripper must not modify the request it is given.
	stripped := req.Clone(req.Context())
	for _, header := range sensitiveHeaders {
		stripped.Header.Del(header)
	}

	return next.RoundTrip(stripped)
}

// isCrossHostRedirect reports whether req was created by http.Client following a redirect that led
// away from the host originally requested any any point in the chain.
//
// It applies the same rule that net/http applies to sensitive headers (isDomainOrSubdomain, used by
// shouldCopyHeaderOnRedirect in net/http/client.go): the redirect target must be the original host
// or a subdomain of it. Note that only hostnames are compared, so a redirect to a different port on
// the same host is not a cross-host redirect.
//
// Once sensitive headers have been stripped it keeps them stripped for the rest of the
// chain, even if a later hop leads back to the original host.
func isCrossHostRedirect(req *http.Request) bool {
	if req.Response == nil {
		return false
	}
	originalHost := strings.ToLower(originalRequestHost(req))
	for r := req; r.Response != nil && r.Response.Request != nil; r = r.Response.Request {
		if !isDomainOrSubdomain(strings.ToLower(r.URL.Hostname()), originalHost) {
			return true
		}
	}
	return false
}

// originalRequestHost walks back through the chain to return the original request Host.
func originalRequestHost(req *http.Request) string {
	r := req
	for r.Response != nil && r.Response.Request != nil {
		r = r.Response.Request
	}
	return r.URL.Hostname()
}

// isDomainOrSubdomain reports whether sub is a subdomain (or exact
// match) of the parent domain.
//
// Both domains must already be in canonical form.
// Mirrors isDomainOrSubdomain from net/http/client.go.
func isDomainOrSubdomain(sub, parent string) bool {
	if parent == "" {
		return false
	}
	if sub == parent {
		return true
	}
	// If sub contains a :, it's probably an IPv6 address (and is definitely not a hostname).
	// Don't check the suffix in this case, to avoid matching the contents of a IPv6 zone.
	// For example, "::1%.www.example.com" is not a subdomain of "www.example.com".
	if strings.ContainsAny(sub, ":%") {
		return false
	}
	// If sub is "foo.example.com" and parent is "example.com",
	// that means sub must end in "."+parent.
	// Do it without allocating.
	if !strings.HasSuffix(sub, parent) {
		return false
	}
	return sub[len(sub)-len(parent)-1] == '.'
}
