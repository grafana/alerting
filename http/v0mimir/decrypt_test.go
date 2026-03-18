package v0mimir

import (
	"testing"

	commoncfg "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestDecryptHTTPConfig(t *testing.T) {
	cases := []struct {
		name        string
		cfg         *HTTPClientConfig
		secrets     map[string][]byte
		expectedCfg *HTTPClientConfig
		expectedErr string
	}{
		{
			name:        "No secrets, no changes",
			cfg:         &HTTPClientConfig{},
			secrets:     nil,
			expectedCfg: &HTTPClientConfig{},
		},
		{
			name:    "BasicAuth password",
			cfg:     &HTTPClientConfig{},
			secrets: map[string][]byte{"http_config.basic_auth.password": []byte("secret-pass")},
			expectedCfg: &HTTPClientConfig{
				BasicAuth: &BasicAuth{
					Password: "secret-pass",
				},
			},
		},
		{
			name: "BasicAuth password with existing BasicAuth",
			cfg: &HTTPClientConfig{
				BasicAuth: &BasicAuth{
					Username: "user",
				},
			},
			secrets: map[string][]byte{"http_config.basic_auth.password": []byte("secret-pass")},
			expectedCfg: &HTTPClientConfig{
				BasicAuth: &BasicAuth{
					Username: "user",
					Password: "secret-pass",
				},
			},
		},
		{
			name:    "Authorization credentials",
			cfg:     &HTTPClientConfig{},
			secrets: map[string][]byte{"http_config.authorization.credentials": []byte("my-token")},
			expectedCfg: &HTTPClientConfig{
				Authorization: &Authorization{
					Credentials: "my-token",
				},
			},
		},
		{
			name:    "OAuth2 client secret",
			cfg:     &HTTPClientConfig{},
			secrets: map[string][]byte{"http_config.oauth2.client_secret": []byte("oauth-secret")},
			expectedCfg: &HTTPClientConfig{
				OAuth2: &OAuth2{
					ClientSecret: "oauth-secret",
				},
			},
		},
		{
			name:    "Bearer token",
			cfg:     &HTTPClientConfig{},
			secrets: map[string][]byte{"http_config.bearer_token": []byte("bearer-val")},
			expectedCfg: &HTTPClientConfig{
				BearerToken: "bearer-val",
			},
		},
		{
			name:    "TLS config key",
			cfg:     &HTTPClientConfig{},
			secrets: map[string][]byte{"http_config.tls_config.key": []byte("tls-key")},
			expectedCfg: &HTTPClientConfig{
				TLSConfig: TLSConfig{
					Key: "tls-key",
				},
			},
		},
		{
			name:    "OAuth2 TLS config key",
			cfg:     &HTTPClientConfig{},
			secrets: map[string][]byte{"http_config.oauth2.tls_config.key": []byte("oauth-tls-key")},
			expectedCfg: &HTTPClientConfig{
				OAuth2: &OAuth2{
					TLSConfig: TLSConfig{
						Key: "oauth-tls-key",
					},
				},
			},
		},
		{
			name: "All scalar secrets at once",
			cfg:  &HTTPClientConfig{},
			secrets: map[string][]byte{
				"http_config.basic_auth.password":       []byte("pass"),
				"http_config.authorization.credentials": []byte("creds"),
				"http_config.oauth2.client_secret":      []byte("cs"),
				"http_config.bearer_token":              []byte("bt"),
				"http_config.tls_config.key":            []byte("tk"),
				"http_config.oauth2.tls_config.key":     []byte("otk"),
			},
			expectedCfg: &HTTPClientConfig{
				BasicAuth:     &BasicAuth{Password: "pass"},
				Authorization: &Authorization{Credentials: "creds"},
				OAuth2: &OAuth2{
					ClientSecret: "cs",
					TLSConfig:    TLSConfig{Key: "otk"},
				},
				BearerToken: "bt",
				TLSConfig:   TLSConfig{Key: "tk"},
			},
		},
		{
			name: "Does not overwrite when secret is absent",
			cfg: &HTTPClientConfig{
				BasicAuth: &BasicAuth{
					Username: "user",
					Password: "original",
				},
				BearerToken: "original-bearer",
			},
			secrets: nil,
			expectedCfg: &HTTPClientConfig{
				BasicAuth: &BasicAuth{
					Username: "user",
					Password: "original",
				},
				BearerToken: "original-bearer",
			},
		},
		{
			name: "HTTP headers secrets",
			cfg: &HTTPClientConfig{
				HTTPHeaders: Headers{
					"X-Custom": Header{Values: []string{"val"}},
				},
			},
			secrets: map[string][]byte{
				"http_config.http_headers.X-Custom": []byte(`["s1","s2"]`),
			},
			expectedCfg: &HTTPClientConfig{
				HTTPHeaders: Headers{
					"X-Custom": Header{
						Values:  []string{"val"},
						Secrets: []commoncfg.Secret{"s1", "s2"},
					},
				},
			},
		},
		{
			name: "HTTP headers secrets invalid JSON",
			cfg: &HTTPClientConfig{
				HTTPHeaders: Headers{
					"X-Bad": Header{},
				},
			},
			secrets: map[string][]byte{
				"http_config.http_headers.X-Bad": []byte(`not-json`),
			},
			expectedErr: "invalid http_config.http_headers.X-Bad",
		},
		{
			name: "Proxy connect header secrets",
			cfg: &HTTPClientConfig{
				ProxyConfig: ProxyConfig{
					ProxyConnectHeader: ProxyHeader{
						"X-Proxy": []commoncfg.Secret{"old"},
					},
				},
			},
			secrets: map[string][]byte{
				"http_config.proxy_connect_header.X-Proxy": []byte(`["new1","new2"]`),
			},
			expectedCfg: &HTTPClientConfig{
				ProxyConfig: ProxyConfig{
					ProxyConnectHeader: ProxyHeader{
						"X-Proxy": []commoncfg.Secret{"new1", "new2"},
					},
				},
			},
		},
		{
			name: "OAuth2 proxy connect header secrets",
			cfg: &HTTPClientConfig{
				OAuth2: &OAuth2{
					ClientID: "id",
					ProxyConfig: ProxyConfig{
						ProxyConnectHeader: ProxyHeader{
							"X-Auth": []commoncfg.Secret{"old"},
						},
					},
				},
			},
			secrets: map[string][]byte{
				"http_config.oauth2.proxy_connect_header.X-Auth": []byte(`["new"]`),
			},
			expectedCfg: &HTTPClientConfig{
				OAuth2: &OAuth2{
					ClientID: "id",
					ProxyConfig: ProxyConfig{
						ProxyConnectHeader: ProxyHeader{
							"X-Auth": []commoncfg.Secret{"new"},
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := DecryptHTTPConfig("http_config", c.cfg, receiversTesting.DecryptForTesting(c.secrets))

			if c.expectedErr != "" {
				require.ErrorContains(t, err, c.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectedCfg, actual)
		})
	}
}
