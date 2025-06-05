package http

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
)

func TestClient(t *testing.T) {
	t.Run("NewClient", func(t *testing.T) {
		client := NewClient()
		require.NotNil(t, client)
	})

	t.Run("WithUserAgent", func(t *testing.T) {
		client := NewClient(WithUserAgent("TEST"))
		require.Equal(t, "TEST", client.cfg.userAgent)
	})

	t.Run("WithDialer with timeout", func(t *testing.T) {
		dialer := net.Dialer{Timeout: 5 * time.Second}
		client := NewClient(WithDialer(dialer))
		require.Equal(t, dialer, client.cfg.dialer)
	})

	t.Run("WithDialer missing timeout should use default", func(t *testing.T) {
		// Mostly defensive to ensure that some timeout is set.
		dialer := net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP("::")}}
		client := NewClient(WithDialer(dialer))

		expectedDialer := dialer
		expectedDialer.Timeout = defaultDialTimeout
		require.Equal(t, expectedDialer, client.cfg.dialer)
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
	s := NewClient(WithUserAgent("TEST"))

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

		client := NewClient()
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

		err := client.SendWebhook(context.Background(), log.NewNopLogger(), webhook)
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

		client := NewClient()
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
