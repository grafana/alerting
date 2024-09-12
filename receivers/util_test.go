package receivers

import (
	"testing"

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
			tlsCfg, err := tt.cfg.ToTLSConfig()

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
