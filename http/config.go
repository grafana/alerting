package http

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"golang.org/x/net/http/httpproxy"

	"github.com/grafana/alerting/receivers"
)

var (
	// ErrInvalidOAuth2Config is returned when the OAuth2 configuration is invalid.
	ErrInvalidOAuth2Config = fmt.Errorf("invalid OAuth2 configuration")
	// ErrInvalidProxyConfig is returned when the proxy configuration is invalid.
	ErrInvalidProxyConfig = fmt.Errorf("invalid proxy configuration")
)

// HTTPClientConfig holds common configuration for notifier HTTP clients.
//
//nolint:revive
type HTTPClientConfig struct {
	// The OAuth2 client credentials used to fetch a token for the targets.
	OAuth2 *OAuth2Config `yaml:"oauth2,omitempty" json:"oauth2,omitempty"`
}

// Decrypt decrypts sensitive fields in the HTTPClientConfig.
func (c *HTTPClientConfig) Decrypt(decryptFn receivers.DecryptFunc) {
	if c.OAuth2 == nil {
		return
	}
	c.OAuth2.ClientSecret = decryptFn("http_config.oauth2.client_secret", c.OAuth2.ClientSecret)

	if c.OAuth2.TLSConfig == nil {
		return
	}
	c.OAuth2.TLSConfig.CACertificate = decryptFn("http_config.oauth2.tls_config.caCertificate", c.OAuth2.TLSConfig.CACertificate)
	c.OAuth2.TLSConfig.ClientCertificate = decryptFn("http_config.oauth2.tls_config.clientCertificate", c.OAuth2.TLSConfig.ClientCertificate)
	c.OAuth2.TLSConfig.ClientKey = decryptFn("http_config.oauth2.tls_config.clientKey", c.OAuth2.TLSConfig.ClientKey)
}

func ValidateHTTPClientConfig(cfg *HTTPClientConfig) error {
	if cfg != nil {
		if err := ValidateOAuth2Config(cfg.OAuth2); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidOAuth2Config, err)
		}
	}
	return nil
}

type ProxyConfig struct {
	// ProxyURL is the HTTP proxy server to use to connect to the targets.
	ProxyURL string `yaml:"proxy_url,omitempty" json:"proxy_url,omitempty"`
	// NoProxy contains addresses that should not use a proxy.
	NoProxy string `yaml:"no_proxy,omitempty" json:"no_proxy,omitempty"`
	// ProxyFromEnvironment uses environment HTTP_PROXY, HTTPS_PROXY and NO_PROXY to determine proxies.
	ProxyFromEnvironment bool `yaml:"proxy_from_environment,omitempty" json:"proxy_from_environment,omitempty"`
	// ProxyConnectHeader optionally specifies headers to send to proxies during CONNECT requests.
	ProxyConnectHeader map[string]string `yaml:"proxy_connect_header,omitempty" json:"proxy_connect_header,omitempty"`
}

// Proxy returns the Proxy URL for a request.
func (cfg *ProxyConfig) Proxy() (fn func(*http.Request) (*url.URL, error), err error) {
	if cfg == nil {
		return nil, nil
	}
	if cfg.ProxyFromEnvironment {
		proxyFn := httpproxy.FromEnvironment().ProxyFunc()
		return func(req *http.Request) (*url.URL, error) {
			return proxyFn(req.URL)
		}, nil
	}
	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", cfg.ProxyURL, err)
		}
		if cfg.NoProxy == "" {
			return http.ProxyURL(proxyURL), nil
		}
		proxy := &httpproxy.Config{
			HTTPProxy:  proxyURL.String(),
			HTTPSProxy: proxyURL.String(),
			NoProxy:    cfg.NoProxy,
		}
		proxyFn := proxy.ProxyFunc()
		return func(req *http.Request) (*url.URL, error) {
			return proxyFn(req.URL)
		}, nil
	}
	return nil, nil
}

func (cfg *ProxyConfig) GetProxyConnectHeader() http.Header {
	if cfg == nil || len(cfg.ProxyConnectHeader) == 0 {
		return nil
	}
	// Return a copy of the header to avoid modifying the original.
	headerCopy := make(http.Header, len(cfg.ProxyConnectHeader))
	for k, v := range cfg.ProxyConnectHeader {
		headerCopy.Add(k, v)
	}
	return headerCopy
}

func ValidateProxyConfig(cfg *ProxyConfig) error {
	if cfg == nil {
		// If no proxy config is provided, we consider it valid.
		return nil
	}
	if len(cfg.ProxyConnectHeader) > 0 && !cfg.ProxyFromEnvironment && cfg.ProxyURL == "" {
		return errors.New("if proxy_connect_header is configured, proxy_url or proxy_from_environment must also be configured")
	}
	if cfg.ProxyFromEnvironment && cfg.ProxyURL != "" {
		return errors.New("if proxy_from_environment is configured, proxy_url must not be configured")
	}
	if cfg.ProxyFromEnvironment && cfg.NoProxy != "" {
		return errors.New("if proxy_from_environment is configured, no_proxy must not be configured")
	}
	if cfg.ProxyURL == "" && cfg.NoProxy != "" {
		return errors.New("if no_proxy is configured, proxy_url must also be configured")
	}

	if cfg.ProxyURL != "" {
		if _, err := url.Parse(cfg.ProxyURL); err != nil {
			return fmt.Errorf("invalid proxy URL %q: %w", cfg.ProxyURL, err)
		}
	}

	return nil
}
