package http

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
)

func TestValidateOAuth2Config(t *testing.T) {
	tests := []struct {
		name    string
		config  OAuth2Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: OAuth2Config{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				TokenURL:     "https://example.com/token",
			},
			wantErr: false,
		},
		{
			name: "missing client ID",
			config: OAuth2Config{
				ClientSecret: "client-secret",
				TokenURL:     "https://example.com/token",
			},
			wantErr: true,
		},
		{
			name: "missing client secret",
			config: OAuth2Config{
				ClientID: "client-id",
				TokenURL: "https://example.com/token",
			},
			wantErr: true,
		},
		{
			name: "missing token URL",
			config: OAuth2Config{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
			},
			wantErr: true,
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
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOAuth2Config(tt.config)
			if tt.wantErr {
				require.Errorf(t, err, "ValidateOAuth2Config() expected error, got nil")
				return
			}
			require.NoError(t, err, "ValidateOAuth2Config() expected nil error, got %v", err)
		})
	}
}
