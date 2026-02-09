package receivers

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test certificates from https://github.com/golang/go/blob/4f852b9734249c063928b34a02dd689e03a8ab2c/src/crypto/tls/tls_test.go#L34
const (
	testRsaCertPem = `-----BEGIN CERTIFICATE-----
MIIB0zCCAX2gAwIBAgIJAI/M7BYjwB+uMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTIwOTEyMjE1MjAyWhcNMTUwOTEyMjE1MjAyWjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANLJ
hPHhITqQbPklG3ibCVxwGMRfp/v4XqhfdQHdcVfHap6NQ5Wok/4xIA+ui35/MmNa
rtNuC+BdZ1tMuVCPFZcCAwEAAaNQME4wHQYDVR0OBBYEFJvKs8RfJaXTH08W+SGv
zQyKn0H8MB8GA1UdIwQYMBaAFJvKs8RfJaXTH08W+SGvzQyKn0H8MAwGA1UdEwQF
MAMBAf8wDQYJKoZIhvcNAQEFBQADQQBJlffJHybjDGxRMqaRmDhX0+6v02TUKZsW
r5QuVbpQhH6u+0UgcW0jp9QwpxoPTLTWGXEWBBBurxFwiCBhkQ+V
-----END CERTIFICATE-----`

	testRsaKeyPem = `-----BEGIN RSA PRIVATE KEY-----
MIIBOwIBAAJBANLJhPHhITqQbPklG3ibCVxwGMRfp/v4XqhfdQHdcVfHap6NQ5Wo
k/4xIA+ui35/MmNartNuC+BdZ1tMuVCPFZcCAwEAAQJAEJ2N+zsR0Xn8/Q6twa4G
6OB1M1WO+k+ztnX/1SvNeWu8D6GImtupLTYgjZcHufykj09jiHmjHx8u8ZZB/o1N
MQIhAPW+eyZo7ay3lMz1V01WVjNKK9QSn1MJlb06h/LuYv9FAiEA25WPedKgVyCW
SmUwbPw8fnTcpqDWE3yTO3vKcebqMSsCIBF3UmVue8YU3jybC3NxuXq3wNm34R8T
xVLHwDXh/6NJAiEAl2oHGGLz64BuAfjKrqwz7qMYr9HCLIe/YsoWq/olzScCIQDi
D2lWusoe2/nEqfDVVWGWlyJ7yOmqaVm/iNUN9B2N2g==
-----END RSA PRIVATE KEY-----`
)

func TestNewTLSConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         TLSConfig
		expectError bool
	}{
		{
			name:        "empty TLSConfig",
			cfg:         TLSConfig{},
			expectError: false,
		},
		{
			name: "valid CA certificate",
			cfg: TLSConfig{
				CACertificate: string(testRsaCertPem),
			},
			expectError: false,
		},
		{
			name: "invalid CA certificate",
			cfg: TLSConfig{
				CACertificate: "invalid-cert",
			},
			expectError: true,
		},
		{
			name: "valid client certificate and key",
			cfg: TLSConfig{
				ClientCertificate: string(testRsaCertPem),
				ClientKey:         string(testRsaKeyPem),
			},
			expectError: false,
		},
		{
			name: "invalid client certificate",
			cfg: TLSConfig{
				ClientCertificate: string(testRsaCertPem),
			},
			expectError: true,
		},
		{
			name: "set InsecureSkipVerify and ServerName",
			cfg: TLSConfig{
				InsecureSkipVerify: true,
				ServerName:         "example.com",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsCfg, err := tt.cfg.ToCryptoTLSConfig()

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, tlsCfg)
			} else {
				require.NoError(t, err)

				require.Equal(t, tt.cfg.InsecureSkipVerify, tlsCfg.InsecureSkipVerify, "InsecureSkipVerify mismatch")
				require.Equal(t, tt.cfg.ServerName, tlsCfg.ServerName, "ServerName mismatch")

				if tt.cfg.CACertificate != "" {
					require.NotNil(t, tlsCfg.RootCAs, "expected RootCAs to be initialized, but it was nil")
				}

				if tt.cfg.ClientCertificate != "" && tt.cfg.ClientKey != "" {
					require.NotEmpty(t, tlsCfg.Certificates, "expected Certificates to be set, but it was empty")
				}
			}
		})
	}
}

type connCounter struct {
	net.Listener
	active int32
}

func (c *connCounter) Accept() (net.Conn, error) {
	conn, err := c.Listener.Accept()
	if err != nil {
		return nil, err
	}

	atomic.AddInt32(&c.active, 1)
	return &countedConn{Conn: conn, counter: &c.active}, nil
}

type countedConn struct {
	net.Conn
	counter *int32
	closed  bool
}

func (c *countedConn) Close() error {
	if !c.closed {
		atomic.AddInt32(c.counter, -1)
		c.closed = true
	}
	return c.Conn.Close()
}

func TestSenderDisablesKeepAlives(t *testing.T) {
	// Create a listener that tracks connections
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	counter := &connCounter{Listener: l}

	// Create test server with our counting listener
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Listener = counter
	srv.Start()
	defer srv.Close()

	// Parse server URL
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	// Create sender
	s := NewSender(log.NewNopLogger())

	// Make multiple requests
	for range 3 {
		_, err = s.SendHTTPRequest(context.Background(), u, HTTPCfg{})
		require.NoError(t, err)

		// Give a short pause between requests
		time.Sleep(10 * time.Millisecond)
	}

	// Verify no connections are kept alive (activeConns should be 0)
	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&counter.active) == 0
	}, time.Second, 10*time.Millisecond, "connections were kept alive when they should have been closed")
}
