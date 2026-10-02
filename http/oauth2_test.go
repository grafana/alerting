package http

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
)

func TestValidateOAuth2Config(t *testing.T) {
	tests := []struct {
		name     string
		config   OAuth2Config
		expError error
	}{
		{
			name: "valid config",
			config: OAuth2Config{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				TokenURL:     "https://example.com/token",
			},
			expError: nil,
		},
		{
			name: "missing client ID",
			config: OAuth2Config{
				ClientSecret: "client-secret",
				TokenURL:     "https://example.com/token",
			},
			expError: ErrOAuth2ClientIDRequired,
		},
		{
			name: "missing client secret",
			config: OAuth2Config{
				ClientID: "client-id",
				TokenURL: "https://example.com/token",
			},
			expError: ErrOAuth2ClientSecretRequired,
		},
		{
			name: "missing token URL",
			config: OAuth2Config{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
			},
			expError: ErrOAuth2TokenURLRequired,
		},
		{
			name: "invalid TLS config",
			config: OAuth2Config{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				TokenURL:     "https://example.com/token",
				TLSConfig: &receivers.TLSConfig{
					CACertificate: "invalid-cert",
				},
			},
			expError: ErrOAuth2TLSConfigInvalid,
		},
		{
			name: "invalid proxy config",
			config: OAuth2Config{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				TokenURL:     "https://example.com/token",
				ProxyConfig: &ProxyConfig{
					NoProxy: "localhost",
				},
			},
			expError: ErrInvalidProxyConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOAuth2Config(&tt.config)
			if tt.expError != nil {
				require.ErrorIs(t, err, tt.expError, "ValidateOAuth2Config() expected error %v, got %v", tt.expError, err)
				return
			}
			require.NoError(t, err, "ValidateOAuth2Config() expected nil error, got %v", err)
		})
	}
}

const (
	testOAuth2ClientID     = "test-client-id"
	testOAuth2ClientSecret = "test-client-secret"
	testOAuth2AccessToken  = "super-secret-access-token"
	testOAuth2AuthHeader   = "Bearer " + testOAuth2AccessToken
)

// TestSendWebhookOAuth2Redirect asserts that the OAuth2 access token minted for a webhook is not
// leaked to a different host when the webhook responds with a redirect.
//
// net/http already strips the Authorization header from a request that is redirected to another
// host (see shouldCopyHeaderOnRedirect in net/http/client.go), but oauth2.Transport re-adds it
// unconditionally on every hop, so anything the token is attached to by the transport chain rather
// than by the caller survives that stripping.
func TestSendWebhookOAuth2Redirect(t *testing.T) {
	send := func(t *testing.T, hostname string) *redirectTargetServer {
		t.Helper()

		tokenURL, tokenRequests := startOAuth2TokenServer(t)
		target := startRedirectTarget(t)

		webhookRequests := 0
		webhookURL := startWebhookServer(t, hostname, func(w http.ResponseWriter, r *http.Request) {
			webhookRequests++
			assert.Equal(t, testOAuth2AuthHeader, r.Header.Get("Authorization"),
				"expected the webhook itself to be authenticated with the access token")
			http.Redirect(w, r, target.URL, http.StatusFound)
		})

		client, err := NewClient(&HTTPClientConfig{
			OAuth2: &OAuth2Config{
				ClientID:     testOAuth2ClientID,
				ClientSecret: testOAuth2ClientSecret,
				TokenURL:     tokenURL,
			},
		})
		require.NoError(t, err, "expected no error creating client")

		err = client.SendWebhook(context.Background(), log.NewNopLogger(), &receivers.SendWebhookSettings{
			URL:        webhookURL,
			Body:       "test-body",
			HTTPMethod: http.MethodPost,
		})
		require.NoError(t, err)

		require.Equal(t, 1, tokenRequests(), "expected exactly one token request")
		require.Equal(t, 1, webhookRequests, "expected exactly one webhook request")
		require.Equal(t, 1, target.requests, "expected the redirect to be followed")

		return target
	}

	t.Run("cross-host redirect must not receive the access token", func(t *testing.T) {
		target := send(t, "localhost")

		require.Empty(t, target.header.Get("Authorization"),
			"the access token must not be sent to a host the request was redirected to")
	})

	t.Run("same-host redirect keeps the access token", func(t *testing.T) {
		target := send(t, "127.0.0.1")

		require.Equal(t, testOAuth2AuthHeader, target.header.Get("Authorization"),
			"the access token should still be sent when the redirect stays on the same host")
	})
}

// startOAuth2TokenServer starts a mock client_credentials token endpoint. It returns the token URL
// and a func reporting how many token requests have been served.
func startOAuth2TokenServer(t *testing.T) (string, func() int) {
	t.Helper()

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++

		assert.Equal(t, GetBasicAuthHeader(testOAuth2ClientID, testOAuth2ClientSecret), r.Header.Get("Authorization"),
			"expected the token request to authenticate with the client credentials")
		assert.NoError(t, r.ParseForm(), "expected no error parsing the token request form")
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token": testOAuth2AccessToken,
			"token_type":   "Bearer",
			"expires_in":   3600,
		}))
	}))
	t.Cleanup(srv.Close)

	return srv.URL + "/oauth2/token", func() int { return requests }
}

// redirectTargetServer stands in for the host a webhook redirects to, and records what it receives.
type redirectTargetServer struct {
	URL      string
	requests int
	header   http.Header
}

// startRedirectTarget starts a redirect target served on 127.0.0.1.
func startRedirectTarget(t *testing.T) *redirectTargetServer {
	t.Helper()

	target := &redirectTargetServer{header: http.Header{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target.requests++
		target.header = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	target.URL = srv.URL

	return target
}

// startWebhookServer starts a server bound through the given hostname and returns the URL that
// should be used to reach it. Binding and dialling both go through the same name resolution, so
// the URL reaches this server whichever address the hostname resolves to.
//
// Use "localhost" for a server that must look like a different host to a 127.0.0.1 one: net/http
// compares the hostnames of the two URLs when deciding whether a redirect crosses hosts, and
// ignores the port, so two servers on 127.0.0.1 are the same host no matter which ports they use.
func startWebhookServer(t *testing.T, hostname string, handler http.HandlerFunc) string {
	t.Helper()

	ln, err := net.Listen("tcp", net.JoinHostPort(hostname, "0"))
	require.NoError(t, err, "expected no error listening on %s", hostname)

	srv := httptest.NewUnstartedServer(handler)
	require.NoError(t, srv.Listener.Close())
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	return "http://" + net.JoinHostPort(hostname, port) + "/webhook"
}
