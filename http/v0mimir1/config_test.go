// Copyright 2016 The Prometheus Authors
// Modifications Copyright Grafana Labs
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v0mimir1

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	commoncfg "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestTLSConfigValidate(t *testing.T) {
	tests := []struct {
		name   string
		cfg    TLSConfig
		errMsg string
	}{
		{
			name: "valid: empty",
			cfg:  TLSConfig{},
		},
		{
			name: "valid: ca_file only",
			cfg:  TLSConfig{CAFile: "ca.pem"},
		},
		{
			name: "valid: ca inline only",
			cfg:  TLSConfig{CA: "ca-content"},
		},
		{
			name: "valid: ca_ref only",
			cfg:  TLSConfig{CARef: "my-ca-ref"},
		},
		{
			name: "valid: cert and key files",
			cfg:  TLSConfig{CertFile: "cert.pem", KeyFile: "key.pem"},
		},
		{
			name: "valid: cert and key inline",
			cfg:  TLSConfig{Cert: "cert-content", Key: "key-content"},
		},
		{
			name: "valid: cert and key refs",
			cfg:  TLSConfig{CertRef: "cert-ref", KeyRef: "key-ref"},
		},
		{
			name:   "invalid: ca and ca_file both set",
			cfg:    TLSConfig{CA: "ca-content", CAFile: "ca.pem"},
			errMsg: "at most one of ca, ca_file & ca_ref must be configured",
		},
		{
			name:   "invalid: ca and ca_ref both set",
			cfg:    TLSConfig{CA: "ca-content", CARef: "my-ca-ref"},
			errMsg: "at most one of ca, ca_file & ca_ref must be configured",
		},
		{
			name:   "invalid: ca_file and ca_ref both set",
			cfg:    TLSConfig{CAFile: "ca.pem", CARef: "my-ca-ref"},
			errMsg: "at most one of ca, ca_file & ca_ref must be configured",
		},
		{
			name:   "invalid: cert and cert_file both set",
			cfg:    TLSConfig{Cert: "cert-content", CertFile: "cert.pem", KeyFile: "key.pem"},
			errMsg: "at most one of cert, cert_file & cert_ref must be configured",
		},
		{
			name:   "invalid: cert and cert_ref both set",
			cfg:    TLSConfig{Cert: "cert-content", CertRef: "cert-ref", Key: "key-content"},
			errMsg: "at most one of cert, cert_file & cert_ref must be configured",
		},
		{
			name:   "invalid: key and key_file both set",
			cfg:    TLSConfig{CertFile: "cert.pem", Key: "key-content", KeyFile: "key.pem"},
			errMsg: "at most one of key and key_file must be configured",
		},
		{
			name:   "invalid: cert without key",
			cfg:    TLSConfig{CertFile: "cert.pem"},
			errMsg: "exactly one of key or key_file must be configured when a client certificate is configured",
		},
		{
			name:   "invalid: key without cert",
			cfg:    TLSConfig{KeyFile: "key.pem"},
			errMsg: "exactly one of cert or cert_file must be configured when a client key is configured",
		},
		{
			name:   "invalid: cert inline without key",
			cfg:    TLSConfig{Cert: "cert-content"},
			errMsg: "exactly one of key or key_file must be configured when a client certificate is configured",
		},
		{
			name:   "invalid: cert_ref without key",
			cfg:    TLSConfig{CertRef: "cert-ref"},
			errMsg: "exactly one of key or key_file must be configured when a client certificate is configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.errMsg)
			}
		})
	}
}

func TestOAuth2Validate(t *testing.T) {
	validBase := OAuth2{
		ClientID: "client-id",
		TokenURL: "http://example.com/token",
	}

	tests := []struct {
		name   string
		cfg    OAuth2
		errMsg string
	}{
		{
			name: "valid: minimal",
			cfg:  validBase,
		},
		{
			name: "valid: with client_secret",
			cfg:  OAuth2{ClientID: "id", TokenURL: "http://example.com/token", ClientSecret: "secret"},
		},
		{
			name: "valid: with client_secret_file",
			cfg:  OAuth2{ClientID: "id", TokenURL: "http://example.com/token", ClientSecretFile: "file.txt"},
		},
		{
			name: "valid: with client_secret_ref",
			cfg:  OAuth2{ClientID: "id", TokenURL: "http://example.com/token", ClientSecretRef: "my-ref"},
		},
		{
			name:   "invalid: no client_id",
			cfg:    OAuth2{TokenURL: "http://example.com/token"},
			errMsg: "oauth2 client_id must be configured",
		},
		{
			name:   "invalid: no token_url",
			cfg:    OAuth2{ClientID: "client-id"},
			errMsg: "oauth2 token_url must be configured",
		},
		{
			name:   "invalid: client_secret and client_secret_file both set",
			cfg:    OAuth2{ClientID: "id", TokenURL: "http://example.com/token", ClientSecret: "secret", ClientSecretFile: "file.txt"},
			errMsg: "at most one of oauth2 client_secret, client_secret_file & client_secret_ref must be configured",
		},
		{
			name:   "invalid: client_secret and client_secret_ref both set",
			cfg:    OAuth2{ClientID: "id", TokenURL: "http://example.com/token", ClientSecret: "secret", ClientSecretRef: "my-ref"},
			errMsg: "at most one of oauth2 client_secret, client_secret_file & client_secret_ref must be configured",
		},
		{
			name:   "invalid: client_secret_file and client_secret_ref both set",
			cfg:    OAuth2{ClientID: "id", TokenURL: "http://example.com/token", ClientSecretFile: "file.txt", ClientSecretRef: "my-ref"},
			errMsg: "at most one of oauth2 client_secret, client_secret_file & client_secret_ref must be configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.errMsg)
			}
		})
	}
}

func TestProxyConfigValidate(t *testing.T) {
	mustParseURL := func(s string) commoncfg.URL {
		u, err := url.Parse(s)
		if err != nil {
			panic(err)
		}
		return commoncfg.URL{URL: u}
	}

	tests := []struct {
		name   string
		cfg    ProxyConfig
		errMsg string
	}{
		{
			name: "valid: empty",
			cfg:  ProxyConfig{},
		},
		{
			name: "valid: proxy_url only",
			cfg:  ProxyConfig{ProxyURL: mustParseURL("http://proxy.example.com")},
		},
		{
			name: "valid: proxy_from_environment only",
			cfg:  ProxyConfig{ProxyFromEnvironment: true},
		},
		{
			name: "valid: proxy_url with no_proxy",
			cfg:  ProxyConfig{ProxyURL: mustParseURL("http://proxy.example.com"), NoProxy: "localhost"},
		},
		{
			name: "valid: proxy_from_environment with proxy_connect_header",
			cfg: ProxyConfig{
				ProxyFromEnvironment: true,
				ProxyConnectHeader:   ProxyHeader{"X-Custom": []commoncfg.Secret{"value"}},
			},
		},
		{
			name: "valid: proxy_url with proxy_connect_header",
			cfg: ProxyConfig{
				ProxyURL:           mustParseURL("http://proxy.example.com"),
				ProxyConnectHeader: ProxyHeader{"X-Custom": []commoncfg.Secret{"value"}},
			},
		},
		{
			name: "invalid: proxy_from_environment and proxy_url both set",
			cfg: ProxyConfig{
				ProxyFromEnvironment: true,
				ProxyURL:             mustParseURL("http://proxy.example.com"),
			},
			errMsg: "if proxy_from_environment is configured, proxy_url must not be configured",
		},
		{
			name: "invalid: proxy_from_environment and no_proxy both set",
			cfg: ProxyConfig{
				ProxyFromEnvironment: true,
				NoProxy:              "localhost",
			},
			errMsg: "if proxy_from_environment is configured, no_proxy must not be configured",
		},
		{
			name: "invalid: no_proxy without proxy_url",
			cfg: ProxyConfig{
				NoProxy: "localhost",
			},
			errMsg: "if no_proxy is configured, proxy_url must also be configured",
		},
		{
			name: "invalid: proxy_connect_header without proxy_url or proxy_from_environment",
			cfg: ProxyConfig{
				ProxyConnectHeader: ProxyHeader{"X-Custom": []commoncfg.Secret{"value"}},
			},
			errMsg: "if proxy_connect_header is configured, proxy_url or proxy_from_environment must also be configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.errMsg)
			}
		})
	}
}

func TestHTTPClientConfigValidate(t *testing.T) {
	tests := []struct {
		name   string
		cfg    HTTPClientConfig
		errMsg string
	}{
		{
			name: "valid: empty",
			cfg:  HTTPClientConfig{},
		},
		{
			name: "valid: basic_auth only",
			cfg:  HTTPClientConfig{BasicAuth: &BasicAuth{Username: "user", Password: "pass"}},
		},
		{
			name: "valid: bearer_token only",
			cfg:  HTTPClientConfig{BearerToken: "token"},
		},
		{
			name: "valid: bearer_token_file only",
			cfg:  HTTPClientConfig{BearerTokenFile: "token.txt"},
		},
		{
			name: "valid: authorization only",
			cfg:  HTTPClientConfig{Authorization: &Authorization{Credentials: "cred"}},
		},
		{
			name: "valid: oauth2 minimal",
			cfg: HTTPClientConfig{
				OAuth2: &OAuth2{ClientID: "id", TokenURL: "http://example.com/token"},
			},
		},
		{
			name: "invalid: bearer_token and bearer_token_file both set",
			cfg: HTTPClientConfig{
				BearerToken:     "token",
				BearerTokenFile: "token.txt",
			},
			errMsg: "at most one of bearer_token & bearer_token_file must be configured",
		},
		{
			name: "invalid: basic_auth and bearer_token both set",
			cfg: HTTPClientConfig{
				BasicAuth:   &BasicAuth{Username: "user"},
				BearerToken: "token",
			},
			errMsg: "at most one of basic_auth, oauth2, bearer_token & bearer_token_file must be configured",
		},
		{
			name: "invalid: oauth2 and bearer_token both set",
			cfg: HTTPClientConfig{
				OAuth2:      &OAuth2{ClientID: "id", TokenURL: "http://example.com/token"},
				BearerToken: "token",
			},
			errMsg: "at most one of basic_auth, oauth2, bearer_token & bearer_token_file must be configured",
		},
		{
			name: "invalid: basic_auth username and username_file both set",
			cfg: HTTPClientConfig{
				BasicAuth: &BasicAuth{Username: "user", UsernameFile: "user.txt"},
			},
			errMsg: "at most one of basic_auth username, username_file & username_ref must be configured",
		},
		{
			name: "invalid: basic_auth username and username_ref both set",
			cfg: HTTPClientConfig{
				BasicAuth: &BasicAuth{Username: "user", UsernameRef: "user-ref"},
			},
			errMsg: "at most one of basic_auth username, username_file & username_ref must be configured",
		},
		{
			name: "invalid: basic_auth password and password_file both set",
			cfg: HTTPClientConfig{
				BasicAuth: &BasicAuth{Password: "pass", PasswordFile: "pass.txt"},
			},
			errMsg: "at most one of basic_auth password, password_file & password_ref must be configured",
		},
		{
			name: "invalid: basic_auth password and password_ref both set",
			cfg: HTTPClientConfig{
				BasicAuth: &BasicAuth{Password: "pass", PasswordRef: "pass-ref"},
			},
			errMsg: "at most one of basic_auth password, password_file & password_ref must be configured",
		},
		{
			name: "invalid: authorization and bearer_token both set",
			cfg: HTTPClientConfig{
				Authorization: &Authorization{Credentials: "cred"},
				BearerToken:   "token",
			},
			errMsg: "authorization is not compatible with bearer_token & bearer_token_file",
		},
		{
			name: "invalid: authorization credentials and credentials_file both set",
			cfg: HTTPClientConfig{
				Authorization: &Authorization{
					Credentials:     "cred",
					CredentialsFile: "cred.txt",
				},
			},
			errMsg: "at most one of authorization credentials & credentials_file must be configured",
		},
		{
			name: "invalid: authorization credentials and credentials_ref both set",
			cfg: HTTPClientConfig{
				Authorization: &Authorization{
					Credentials:    "cred",
					CredentialsRef: "cred-ref",
				},
			},
			errMsg: "at most one of authorization credentials & credentials_file must be configured",
		},
		{
			name: "invalid: authorization type basic",
			cfg: HTTPClientConfig{
				Authorization: &Authorization{Type: "basic"},
			},
			errMsg: `authorization type cannot be set to "basic", use "basic_auth" instead`,
		},
		{
			name: "invalid: basic_auth and authorization both set",
			cfg: HTTPClientConfig{
				BasicAuth:     &BasicAuth{Username: "user"},
				Authorization: &Authorization{Credentials: "cred"},
			},
			errMsg: "at most one of basic_auth, oauth2 & authorization must be configured",
		},
		{
			name: "invalid: oauth2 and authorization both set",
			cfg: HTTPClientConfig{
				OAuth2:        &OAuth2{ClientID: "id", TokenURL: "http://example.com/token"},
				Authorization: &Authorization{Credentials: "cred"},
			},
			errMsg: "at most one of basic_auth, oauth2 & authorization must be configured",
		},
		{
			name: "invalid: basic_auth and oauth2 both set",
			cfg: HTTPClientConfig{
				BasicAuth: &BasicAuth{Username: "user"},
				OAuth2:    &OAuth2{ClientID: "id", TokenURL: "http://example.com/token"},
			},
			errMsg: "at most one of basic_auth, oauth2 & authorization must be configured",
		},
		{
			name: "invalid: http_headers with reserved header",
			cfg: HTTPClientConfig{
				HTTPHeaders: &Headers{
					Headers: map[string]Header{
						"User-Agent": {Values: []string{"custom-agent"}},
					},
				},
			},
			errMsg: `setting header "User-Agent" is not allowed`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.errMsg)
			}
		})
	}
}

func TestHeadersValidate(t *testing.T) {
	tests := []struct {
		name   string
		cfg    Headers
		errMsg string
	}{
		{
			name: "valid: empty",
			cfg:  Headers{},
		},
		{
			name: "valid: custom header",
			cfg:  Headers{Headers: map[string]Header{"X-Custom": {Values: []string{"val"}}}},
		},
		{
			name:   "invalid: Authorization header",
			cfg:    Headers{Headers: map[string]Header{"Authorization": {Values: []string{"Bearer token"}}}},
			errMsg: `setting header "Authorization" is not allowed`,
		},
		{
			name:   "invalid: User-Agent header",
			cfg:    Headers{Headers: map[string]Header{"User-Agent": {Values: []string{"custom"}}}},
			errMsg: `setting header "User-Agent" is not allowed`,
		},
		{
			name:   "invalid: Content-Type header",
			cfg:    Headers{Headers: map[string]Header{"Content-Type": {Values: []string{"application/json"}}}},
			errMsg: `setting header "Content-Type" is not allowed`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.errMsg)
			}
		})
	}
}

func TestReservedHeadersAreCanonical(t *testing.T) {
	for k := range ReservedHeaders {
		canonical := http.CanonicalHeaderKey(k)
		require.Equalf(t, canonical, k, "ReservedHeaders key %q is not canonical, expected %q", k, canonical)
	}
}

func TestProxyHeaderHTTPHeader(t *testing.T) {
	tests := map[string]struct {
		header   ProxyHeader
		expected http.Header
	}{
		"basic": {
			header: ProxyHeader{
				"single": []commoncfg.Secret{"v1"},
				"multi":  []commoncfg.Secret{"v1", "v2"},
				"empty":  []commoncfg.Secret{},
				"nil":    nil,
			},
			expected: http.Header{
				"single": []string{"v1"},
				"multi":  []string{"v1", "v2"},
				"empty":  []string{},
				"nil":    nil,
			},
		},
		"nil": {
			header:   nil,
			expected: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := tc.header.HTTPHeader()
			require.Truef(t, reflect.DeepEqual(actual, tc.expected), "expecting: %#v, actual: %#v", tc.expected, actual)
		})
	}
}

func TestHTTPClientConfigUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		check  func(t *testing.T, cfg *HTTPClientConfig)
		errMsg string
	}{
		{
			name:  "valid: default values set",
			input: `{}`,
			check: func(t *testing.T, cfg *HTTPClientConfig) {
				require.True(t, cfg.FollowRedirects)
				require.True(t, cfg.EnableHTTP2)
			},
		},
		{
			name: "valid: basic_auth",
			input: `
basic_auth:
  username: user
  password: pass
`,
			check: func(t *testing.T, cfg *HTTPClientConfig) {
				require.NotNil(t, cfg.BasicAuth)
				require.Equal(t, "user", cfg.BasicAuth.Username)
				require.Equal(t, commoncfg.Secret("pass"), cfg.BasicAuth.Password)
			},
		},
		{
			name: "valid: bearer_token converted to authorization",
			input: `
bearer_token: mytoken
`,
			check: func(t *testing.T, cfg *HTTPClientConfig) {
				require.NotNil(t, cfg.Authorization)
				require.Equal(t, commoncfg.Secret("mytoken"), cfg.Authorization.Credentials)
				require.Equal(t, "Bearer", cfg.Authorization.Type)
				require.Empty(t, cfg.BearerToken)
			},
		},
		{
			name: "invalid: bearer_token and bearer_token_file",
			input: `
bearer_token: token
bearer_token_file: token.txt
`,
			errMsg: "at most one of bearer_token & bearer_token_file must be configured",
		},
		{
			name: "invalid: basic_auth and oauth2",
			input: `
basic_auth:
  username: user
oauth2:
  client_id: id
  token_url: http://example.com/token
`,
			errMsg: "at most one of basic_auth, oauth2 & authorization must be configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cfg HTTPClientConfig
			err := yaml.Unmarshal([]byte(tc.input), &cfg)
			if tc.errMsg != "" {
				require.EqualError(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				if tc.check != nil {
					tc.check(t, &cfg)
				}
			}
		})
	}
}

func TestHTTPClientConfigUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		check  func(t *testing.T, cfg *HTTPClientConfig)
		errMsg string
	}{
		{
			name:  "valid: default values set",
			input: `{}`,
			check: func(t *testing.T, cfg *HTTPClientConfig) {
				require.True(t, cfg.FollowRedirects)
				require.True(t, cfg.EnableHTTP2)
			},
		},
		{
			name:  "valid: basic_auth",
			input: `{"basic_auth":{"username":"user","password":"pass"}}`,
			check: func(t *testing.T, cfg *HTTPClientConfig) {
				require.NotNil(t, cfg.BasicAuth)
				require.Equal(t, "user", cfg.BasicAuth.Username)
				require.Equal(t, commoncfg.Secret("pass"), cfg.BasicAuth.Password)
			},
		},
		{
			name:  "valid: bearer_token converted to authorization",
			input: `{"bearer_token":"mytoken"}`,
			check: func(t *testing.T, cfg *HTTPClientConfig) {
				require.NotNil(t, cfg.Authorization)
				require.Equal(t, commoncfg.Secret("mytoken"), cfg.Authorization.Credentials)
				require.Equal(t, "Bearer", cfg.Authorization.Type)
				require.Empty(t, cfg.BearerToken)
			},
		},
		{
			name:   "invalid: bearer_token and bearer_token_file",
			input:  `{"bearer_token":"token","bearer_token_file":"token.txt"}`,
			errMsg: "at most one of bearer_token & bearer_token_file must be configured",
		},
		{
			name:   "invalid: oauth2 no client_id",
			input:  `{"oauth2":{"token_url":"http://example.com/token","client_id":""}}`,
			errMsg: "oauth2 client_id must be configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cfg HTTPClientConfig
			err := json.Unmarshal([]byte(tc.input), &cfg)
			if tc.errMsg != "" {
				require.EqualError(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				if tc.check != nil {
					tc.check(t, &cfg)
				}
			}
		})
	}
}

func TestTLSConfigUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errMsg string
	}{
		{
			name:  "valid: empty",
			input: `{}`,
		},
		{
			name:  "valid: ca_file",
			input: `ca_file: /etc/ca.pem`,
		},
		{
			name:  "valid: cert_file and key_file",
			input: "cert_file: cert.pem\nkey_file: key.pem",
		},
		{
			name:   "invalid: ca and ca_file both set",
			input:  "ca: ca-content\nca_file: ca.pem",
			errMsg: "at most one of ca, ca_file & ca_ref must be configured",
		},
		{
			name:   "invalid: cert without key",
			input:  "cert_file: cert.pem",
			errMsg: "exactly one of key or key_file must be configured when a client certificate is configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cfg TLSConfig
			err := yaml.Unmarshal([]byte(tc.input), &cfg)
			if tc.errMsg != "" {
				require.EqualError(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOAuth2UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errMsg string
	}{
		{
			name:  "valid: minimal",
			input: "client_id: id\ntoken_url: http://example.com/token",
		},
		{
			name:   "invalid: no client_id",
			input:  "token_url: http://example.com/token",
			errMsg: "oauth2 client_id must be configured",
		},
		{
			name:   "invalid: no token_url",
			input:  "client_id: id",
			errMsg: "oauth2 token_url must be configured",
		},
		{
			name:   "invalid: client_secret and client_secret_file",
			input:  "client_id: id\ntoken_url: http://example.com/token\nclient_secret: s\nclient_secret_file: f.txt",
			errMsg: "at most one of oauth2 client_secret, client_secret_file & client_secret_ref must be configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cfg OAuth2
			err := yaml.Unmarshal([]byte(tc.input), &cfg)
			if tc.errMsg != "" {
				require.EqualError(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
