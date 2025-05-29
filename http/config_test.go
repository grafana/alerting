package http

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPClientConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *HTTPClientConfig
		expError error
	}{
		{
			name: "valid config with OAuth2",
			cfg: &HTTPClientConfig{
				OAuth2: &OAuth2Config{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					TokenURL:     "https://example.com/token",
				},
			},
			expError: nil,
		},
		{
			name:     "nil config",
			cfg:      nil,
			expError: nil,
		},
		{
			name: "invalid OAuth2 config",
			cfg: &HTTPClientConfig{
				OAuth2: &OAuth2Config{
					ClientID: "",
				},
			},
			expError: ErrInvalidOAuth2Config,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHTTPClientConfig(tt.cfg)
			if tt.expError != nil {
				require.ErrorIs(t, err, tt.expError)
				return
			}
			require.NoError(t, err, "expected no error for valid config")
		})
	}
}

func TestProxyConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *ProxyConfig
		wantErr bool
	}{
		{
			name: "valid proxy config with URL",
			cfg: &ProxyConfig{
				ProxyURL: "http://proxy.example.com:8080",
			},
			wantErr: false,
		},
		{
			name:    "empty proxy config",
			cfg:     nil,
			wantErr: false,
		},
		{
			name: "invalid proxy URL",
			cfg: &ProxyConfig{
				ProxyURL: "ht tp://l ocalhost :12 34",
			},
			wantErr: true,
		},
		{
			name: "invalid proxy URL and environment",
			cfg: &ProxyConfig{
				ProxyURL:             "http://proxy.example.com:8080",
				ProxyFromEnvironment: true,
			},
			wantErr: true,
		},
		{
			name: "invalid proxy environment with no proxy",
			cfg: &ProxyConfig{
				ProxyFromEnvironment: true,
				NoProxy:              "localhost",
			},
			wantErr: true,
		},
		{
			name: "invalid no proxy with empty URL",
			cfg: &ProxyConfig{
				NoProxy: "localhost",
			},
			wantErr: true,
		},
		{
			name: "invalid proxy connect header without URL or environment",
			cfg: &ProxyConfig{
				ProxyConnectHeader: map[string]string{"X-Header": "value"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProxyConfig(tt.cfg)
			if tt.wantErr {
				require.Error(t, err, "expected error for invalid proxy config")
			} else {
				require.NoError(t, err, "expected no error for valid proxy config")
			}
		})
	}
}
