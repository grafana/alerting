package http

import (
	"testing"

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
				ProxyConfig: ProxyConfig{
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
