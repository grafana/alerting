package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
)

func TestClient(t *testing.T) {
	t.Run("NewClient", func(t *testing.T) {
		client, err := NewClient()
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("WithUserAgent", func(t *testing.T) {
		client, err := NewClient(WithUserAgent("TEST"))
		require.NoError(t, err)
		require.Equal(t, "TEST", client.cfg.userAgent)
	})

	t.Run("WithDialer with timeout", func(t *testing.T) {
		dialer := net.Dialer{Timeout: 5 * time.Second}
		client, err := NewClient(WithDialer(dialer))
		require.NoError(t, err)
		require.Equal(t, dialer, client.cfg.dialer)
	})

	t.Run("WithDialer missing timeout should use default", func(t *testing.T) {
		// Mostly defensive to ensure that some timeout is set.
		dialer := net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP("::")}}
		client, err := NewClient(WithDialer(dialer))
		require.NoError(t, err)

		expectedDialer := dialer
		expectedDialer.Timeout = defaultDialTimeout
		require.Equal(t, expectedDialer, client.cfg.dialer)
	})

	t.Run("WithOAuth2", func(t *testing.T) {
		oauth2Config := &OAuth2Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			TokenURL:     "https://localhost:8080/oauth2/token",
		}
		client, err := NewClient(WithOAuth2(oauth2Config))
		require.NoError(t, err)

		require.Equal(t, oauth2Config, client.cfg.ouath2Config)
	})

	t.Run("WithOAuth2 invalid TLS", func(t *testing.T) {
		oauth2Config := &OAuth2Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			TokenURL:     "https://localhost:8080/oauth2/token",
			TLSConfig: &receivers.TLSConfig{
				CACertificate: "invalid-ca-cert",
			},
		}
		_, err := NewClient(WithOAuth2(oauth2Config))
		require.ErrorIs(t, err, ErrOAuth2TLSConfigInvalid)
	})
}

func TestSendWebhook(t *testing.T) {
	var got *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		got = r
		w.WriteHeader(http.StatusOK)
	}))
	s, err := NewClient(WithUserAgent("TEST"))
	require.NoError(t, err)

	// The method should be either POST or PUT.
	cmd := receivers.SendWebhookSettings{
		HTTPMethod: http.MethodGet,
		URL:        server.URL,
	}
	require.ErrorIs(t, s.SendWebhook(context.Background(), log.NewNopLogger(), &cmd), ErrInvalidMethod)

	// If the method is not specified, it should default to POST.
	// Content type should default to application/json.
	testHeaders := map[string]string{
		"test-header-1": "test-1",
		"test-header-2": "test-2",
		"test-header-3": "test-3",
	}
	cmd = receivers.SendWebhookSettings{
		URL:        server.URL,
		HTTPHeader: testHeaders,
	}
	require.NoError(t, s.SendWebhook(context.Background(), log.NewNopLogger(), &cmd))
	require.Equal(t, http.MethodPost, got.Method)
	require.Equal(t, "application/json", got.Header.Get("Content-Type"))

	// User agent should be correctly set.
	require.Equal(t, "TEST", got.Header.Get("User-Agent"))

	// No basic auth should be set if user and password are not provided.
	_, _, ok := got.BasicAuth()
	require.False(t, ok)

	// Request heders should be set.
	for k, v := range testHeaders {
		require.Equal(t, v, got.Header.Get(k))
	}

	// Basic auth should be correctly set.
	testUser := "test-user"
	testPassword := "test-password"
	cmd = receivers.SendWebhookSettings{
		URL:      server.URL,
		User:     testUser,
		Password: testPassword,
	}

	require.NoError(t, s.SendWebhook(context.Background(), log.NewNopLogger(), &cmd))
	user, password, ok := got.BasicAuth()
	require.True(t, ok)
	require.Equal(t, testUser, user)
	require.Equal(t, testPassword, password)

	// Validation errors should be returned.
	testErr := errors.New("test")
	cmd = receivers.SendWebhookSettings{
		URL:        server.URL,
		Validation: func([]byte, int) error { return testErr },
	}

	require.ErrorIs(t, s.SendWebhook(context.Background(), log.NewNopLogger(), &cmd), testErr)

	// A non-200 status code should cause an error.
	cmd = receivers.SendWebhookSettings{
		URL: server.URL + "/error",
	}
	require.Error(t, s.SendWebhook(context.Background(), log.NewNopLogger(), &cmd))
}

func TestSendWebhookHMAC(t *testing.T) {
	var capturedRequest *http.Request

	initServer := func(serverType func(http.Handler) *httptest.Server) *httptest.Server {
		capturedRequest = nil
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedRequest = r
			w.WriteHeader(http.StatusOK)
		})
		server := serverType(handler)
		return server
	}

	t.Run("with plain HTTP", func(t *testing.T) {
		server := initServer(httptest.NewServer)
		defer server.Close()

		client, err := NewClient()
		require.NoError(t, err)
		webhook := &receivers.SendWebhookSettings{
			URL:        server.URL,
			Body:       "test-body",
			HTTPMethod: http.MethodPost,
			HMACConfig: &receivers.HMACConfig{
				Secret:          "test-secret",
				Header:          "X-Custom-HMAC",
				TimestampHeader: "X-Custom-Timestamp",
			},
		}

		err = client.SendWebhook(context.Background(), log.NewNopLogger(), webhook)
		require.NoError(t, err)

		require.NotNil(t, capturedRequest)
		require.NotEmpty(t, capturedRequest.Header.Get("X-Custom-HMAC"))
		timestamp := capturedRequest.Header.Get("X-Custom-Timestamp")
		require.NotEmpty(t, timestamp)
	})

	t.Run("with TLS", func(t *testing.T) {
		server := initServer(httptest.NewTLSServer)
		defer server.Close()

		tlsConfig := &receivers.TLSConfig{
			InsecureSkipVerify: true,
		}
		cfg, err := tlsConfig.ToCryptoTLSConfig()
		require.NoError(t, err)

		client, err := NewClient()
		require.NoError(t, err)
		webhook := &receivers.SendWebhookSettings{
			URL:        server.URL,
			Body:       "test-body",
			HTTPMethod: http.MethodPost,
			TLSConfig:  cfg,
			HMACConfig: &receivers.HMACConfig{
				Secret:          "test-secret",
				Header:          "X-Custom-HMAC",
				TimestampHeader: "X-Custom-Timestamp",
			},
		}

		err = client.SendWebhook(context.Background(), log.NewNopLogger(), webhook)
		require.NoError(t, err)

		require.NotNil(t, capturedRequest)
		require.NotEmpty(t, capturedRequest.Header.Get("X-Custom-HMAC"))
		timestamp := capturedRequest.Header.Get("X-Custom-Timestamp")
		require.NotEmpty(t, timestamp)
	})
}

func TestSendWebhookOAuth2(t *testing.T) {
	type oauth2Response struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}

	customDialError := fmt.Errorf("custom dial function error")
	tcs := []struct {
		name            string
		oauth2Config    OAuth2Config // TokenURL is added dynamically in the test.
		otherClientOpts []ClientOption
		oauth2Response  oauth2Response

		expOAuth2AuthHeaders   http.Header
		expOAuth2RequestValues url.Values
		expClientError         error
		expOAuthError          error
	}{
		{
			name: "valid simple OAuth2 config",
			oauth2Config: OAuth2Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			oauth2Response: oauth2Response{
				AccessToken: "12345",
				TokenType:   "Bearer",
			},

			expOAuth2RequestValues: url.Values{
				"grant_type": []string{"client_credentials"},
			},
			expOAuth2AuthHeaders: http.Header{
				"Authorization": []string{GetBasicAuthHeader("test-client-id", "test-client-secret")},
			},
		},
		{
			name: "client with useragent should use in oauth2 request",
			oauth2Config: OAuth2Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			otherClientOpts: []ClientOption{WithUserAgent("TEST")},
			oauth2Response: oauth2Response{
				AccessToken: "12345",
				TokenType:   "Bearer",
			},

			expOAuth2RequestValues: url.Values{
				"grant_type": []string{"client_credentials"},
			},
			expOAuth2AuthHeaders: http.Header{
				"Authorization": []string{GetBasicAuthHeader("test-client-id", "test-client-secret")},
				"User-Agent":    []string{"TEST"},
			},
		},
		{
			name: "valid with scopes",
			oauth2Config: OAuth2Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				Scopes:       []string{"scope1", "scope2"},
			},
			oauth2Response: oauth2Response{
				AccessToken: "12345",
				TokenType:   "Bearer",
			},

			expOAuth2RequestValues: url.Values{
				"grant_type": []string{"client_credentials"},
				"scope":      []string{"scope1 scope2"},
			},
			expOAuth2AuthHeaders: http.Header{
				"Authorization": []string{GetBasicAuthHeader("test-client-id", "test-client-secret")},
			},
		},
		{
			name: "valid with endpoint params",
			oauth2Config: OAuth2Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				EndpointParams: map[string]string{
					"audience": "test-audience",
				},
			},
			oauth2Response: oauth2Response{
				AccessToken: "12345",
				TokenType:   "Bearer",
			},

			expOAuth2RequestValues: url.Values{
				"grant_type": []string{"client_credentials"},
				"audience":   []string{"test-audience"},
			},
			expOAuth2AuthHeaders: http.Header{
				"Authorization": []string{GetBasicAuthHeader("test-client-id", "test-client-secret")},
			},
		},
		{
			name: "valid with scopes and endpoint params",
			oauth2Config: OAuth2Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				Scopes:       []string{"scope1", "scope2"},
				EndpointParams: map[string]string{
					"audience": "test-audience",
				},
			},
			oauth2Response: oauth2Response{
				AccessToken: "12345",
				TokenType:   "Bearer",
			},

			expOAuth2RequestValues: url.Values{
				"grant_type": []string{"client_credentials"},
				"audience":   []string{"test-audience"},
				"scope":      []string{"scope1 scope2"},
			},
			expOAuth2AuthHeaders: http.Header{
				"Authorization": []string{GetBasicAuthHeader("test-client-id", "test-client-secret")},
			},
		},
		{
			name: "invalid OAuth2 - client id",
			oauth2Config: OAuth2Config{
				ClientSecret: "test-client-secret",
			},
			expClientError: ErrOAuth2ClientIDRequired,
		},
		{
			name: "invalid OAuth2 - client secret",
			oauth2Config: OAuth2Config{
				ClientID: "test-client-id",
			},
			expClientError: ErrOAuth2ClientSecretRequired,
		},
		{
			name: "invalid OAuth2 - tlsConfig",
			oauth2Config: OAuth2Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				TLSConfig: &receivers.TLSConfig{
					CACertificate: "invalid-ca-cert",
				},
			},
			expClientError: ErrOAuth2TLSConfigInvalid,
		},
		{
			name: "client with custom dialer should use in oauth2 request",
			oauth2Config: OAuth2Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			otherClientOpts: []ClientOption{
				WithDialer(net.Dialer{
					// Override the Resolver so that configurations with invalid hostnames also return
					// "custom dial function error" instead of "no such host".
					Resolver: &net.Resolver{
						Dial: func(_ context.Context, _, _ string) (net.Conn, error) {
							return nil, customDialError
						},
					},
					Control: func(_, _ string, _ syscall.RawConn) error {
						return customDialError
					},
				}),
			},
			oauth2Response: oauth2Response{
				AccessToken: "12345",
				TokenType:   "Bearer",
			},

			expOAuth2RequestValues: url.Values{
				"grant_type": []string{"client_credentials"},
			},
			expOAuth2AuthHeaders: http.Header{
				"Authorization": []string{GetBasicAuthHeader("test-client-id", "test-client-secret")},
			},
			expOAuthError: customDialError,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			oathRequestCnt := 0
			oauth2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				oathRequestCnt++

				for k := range tc.expOAuth2AuthHeaders {
					assert.Equalf(t, tc.expOAuth2AuthHeaders.Get(k), r.Header.Get(k), "expected OAuth2 request header %s to match, got: %v", k, r.Header.Get(k))
				}

				err := r.ParseForm()
				assert.NoError(t, err, "expected no error parsing form")

				assert.Equalf(t, tc.expOAuth2RequestValues, r.Form, "expected OAuth2 request values to match, got: %v", r.Form)

				res, _ := json.Marshal(tc.oauth2Response)
				w.Header().Add("Content-Type", "application/json")
				_, _ = w.Write(res)
			}))
			defer oauth2Server.Close()

			webhookRequestCnt := 0
			webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				webhookRequestCnt++
				assert.Equalf(t, tc.oauth2Response.TokenType+" "+tc.oauth2Response.AccessToken, r.Header.Get("Authorization"),
					"expected Authorization header from Access Token to match, got: %s", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
			}))
			defer webhookServer.Close()

			oauthConfig := tc.oauth2Config
			oauthConfig.TokenURL = oauth2Server.URL
			client, err := NewClient(append(tc.otherClientOpts, WithOAuth2(&oauthConfig))...)
			if tc.expClientError != nil {
				assert.ErrorIs(t, err, tc.expClientError, "expected client creation error to match")
				return
			}
			require.NoError(t, err, "expected no error creating client")

			err = client.SendWebhook(context.Background(), log.NewNopLogger(), &receivers.SendWebhookSettings{
				URL:        webhookServer.URL,
				Body:       "test-body",
				HTTPMethod: http.MethodPost,
			})
			if tc.expOAuthError != nil {
				assert.Equal(t, 0, oathRequestCnt, "expected %d OAuth2 request to be sent, got: %d", 1, oathRequestCnt)
				assert.Equal(t, 0, webhookRequestCnt, "expected %d webhook request to be sent, got: %d", 1, webhookRequestCnt)
				assert.ErrorIs(t, err, tc.expOAuthError, "expected error to match")
				return
			}
			assert.Equal(t, 1, oathRequestCnt, "expected %d OAuth2 request to be sent, got: %d", 1, oathRequestCnt)
			assert.Equal(t, 1, webhookRequestCnt, "expected %d webhook request to be sent, got: %d", 1, webhookRequestCnt)
			assert.NoError(t, err, "expected no error")

			// Check that the OAuth2 request is sent only once, and the token is reused.
			_ = client.SendWebhook(context.Background(), log.NewNopLogger(), &receivers.SendWebhookSettings{
				URL:        webhookServer.URL,
				Body:       "test-body",
				HTTPMethod: http.MethodPost,
			})
			assert.Equal(t, 1, oathRequestCnt, "expected %d OAuth2 request to be sent, got: %d", 1, oathRequestCnt)
			assert.Equal(t, 2, webhookRequestCnt, "expected %d webhook request to be sent, got: %d", 2, webhookRequestCnt)
		})
	}
}

func TestToHTTPClientOption(t *testing.T) {
	// this test guards against adding new fields to the configuration structure without updating the conversion function
	t.Run("empty converts to empty", func(t *testing.T) {
		require.Empty(t, ToHTTPClientOption())
		require.Empty(t, ToHTTPClientOption(nil))
	})

	var f ClientOption = func(configuration *clientConfiguration) {
		configuration.userAgent = "test"
		configuration.dialer = net.Dialer{Timeout: 5 * time.Second}
		configuration.customDialer = true
	}
	actual := ToHTTPClientOption(f)
	require.Len(t, actual, 2)

	// Verify number of fields using reflection
	tp := reflect.TypeOf(clientConfiguration{})
	// You need to increase the number of fields covered in this test, if you add a new field to the configuration struct.
	require.Equalf(t, 3, tp.NumField(), "Not all fields are converted to HTTPClientOption, which means that the configuration will not be supported in upstream integrations")
}
