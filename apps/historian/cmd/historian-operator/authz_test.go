package main

import (
	"context"
	"testing"

	authnlib "github.com/grafana/authlib/authn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validAuthzConfig() authzConfig {
	return authzConfig{
		RemoteAddress:    "authz.example.com:10000",
		Token:            "service-token",
		TokenExchangeURL: "https://token-exchange.example.com/exchange",
		TokenNamespace:   "*",
	}
}

func TestNewAccessClient_RequiresConfig(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *authzConfig)
		wantErr string
	}{
		{
			name:    "missing remote address",
			mutate:  func(c *authzConfig) { c.RemoteAddress = "" },
			wantErr: "authz.remote-address is required",
		},
		{
			name:    "missing token",
			mutate:  func(c *authzConfig) { c.Token = "" },
			wantErr: "authz.token and authz.token-exchange-url are required",
		},
		{
			name:    "missing token exchange url",
			mutate:  func(c *authzConfig) { c.TokenExchangeURL = "" },
			wantErr: "authz.token and authz.token-exchange-url are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validAuthzConfig()
			tt.mutate(&cfg)
			client, err := newAccessClient(cfg)
			require.Error(t, err)
			assert.Nil(t, client)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNewAccessClient_BuildsClient(t *testing.T) {
	// grpc.NewClient is lazy (no dial until first RPC), so a valid config yields a
	// usable client without any authz service running.
	client, err := newAccessClient(validAuthzConfig())
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewAccessClient_TLSCertNotFound(t *testing.T) {
	cfg := validAuthzConfig()
	cfg.CertFile = "/does/not/exist.pem"
	client, err := newAccessClient(cfg)
	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to load authz TLS credentials")
}

type fakeTokenExchanger struct {
	gotReq authnlib.TokenExchangeRequest
	token  string
	err    error
}

func (f *fakeTokenExchanger) Exchange(_ context.Context, r authnlib.TokenExchangeRequest) (*authnlib.TokenExchangeResponse, error) {
	f.gotReq = r
	if f.err != nil {
		return nil, f.err
	}
	return &authnlib.TokenExchangeResponse{Token: f.token}, nil
}

func TestTokenAuth_GetRequestMetadata(t *testing.T) {
	exchanger := &fakeTokenExchanger{token: "exchanged-token"}
	ta := newTokenAuth(authzServiceAudience, "stacks-7", exchanger)

	md, err := ta.GetRequestMetadata(context.Background())
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"X-Access-Token": "exchanged-token"}, md)
	assert.Equal(t, "stacks-7", exchanger.gotReq.Namespace)
	assert.Equal(t, []string{authzServiceAudience}, exchanger.gotReq.Audiences)
	assert.False(t, ta.RequireTransportSecurity())
}

func TestTokenAuth_GetRequestMetadata_ExchangeError(t *testing.T) {
	exchanger := &fakeTokenExchanger{err: assert.AnError}
	ta := newTokenAuth(authzServiceAudience, "*", exchanger)

	md, err := ta.GetRequestMetadata(context.Background())
	require.Error(t, err)
	assert.Nil(t, md)
}
