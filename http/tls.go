package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/grafana/alerting/receivers"
)

const (
	defaultMaxIdleConns        = 100
	defaultMaxIdleConnsPerHost = 25
	defaultMaxConnsPerHost     = 25
	defaultMaxResponseDrain    = 64 << 10
)

// TransportConfig controls connection pooling for notifier HTTP clients.
// Zero values use the package defaults for each setting.
type TransportConfig struct {
	// MaxIdleConns limits the total number of idle connections. Zero uses the default.
	MaxIdleConns int
	// MaxIdleConnsPerHost limits idle connections per host. Zero uses the default.
	MaxIdleConnsPerHost int
	// MaxConnsPerHost limits connections per host, including dialing, active, and idle connections. Zero uses the default.
	MaxConnsPerHost int
	// IdleConnTimeout is the maximum time an idle connection remains in the pool. Zero uses the default.
	IdleConnTimeout time.Duration
	// MaxResponseDrainBytes limits how many unread response bytes are discarded before closing the body. Zero uses the default.
	MaxResponseDrainBytes int64
}

// Validate checks that transport pool limits are non-negative.
func (c TransportConfig) Validate() error {
	if c.MaxIdleConns < 0 || c.MaxIdleConnsPerHost < 0 || c.MaxConnsPerHost < 0 || c.IdleConnTimeout < 0 || c.MaxResponseDrainBytes < 0 {
		return fmt.Errorf("HTTP transport pool settings must not be negative")
	}
	return nil
}

var defaultTransportConfig = TransportConfig{
	MaxIdleConns:          defaultMaxIdleConns,
	MaxIdleConnsPerHost:   defaultMaxIdleConnsPerHost,
	MaxConnsPerHost:       defaultMaxConnsPerHost,
	IdleConnTimeout:       90 * time.Second,
	MaxResponseDrainBytes: defaultMaxResponseDrain,
}

func (c TransportConfig) withDefaults() TransportConfig {
	defaults := defaultTransportConfig
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = defaults.MaxIdleConns
	}
	if c.MaxIdleConnsPerHost == 0 {
		c.MaxIdleConnsPerHost = defaults.MaxIdleConnsPerHost
	}
	if c.MaxConnsPerHost == 0 {
		c.MaxConnsPerHost = defaults.MaxConnsPerHost
	}
	if c.IdleConnTimeout == 0 {
		c.IdleConnTimeout = defaults.IdleConnTimeout
	}
	if c.MaxResponseDrainBytes == 0 {
		c.MaxResponseDrainBytes = defaults.MaxResponseDrainBytes
	}
	return c
}

// NewTLSClient creates a new HTTP client with the provided TLS configuration or with default settings.
func NewTLSClient(tlsConfig *tls.Config, dialContextfunc func(context.Context, string, string) (net.Conn, error), transportConfig ...TransportConfig) *http.Client {
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			Renegotiation: tls.RenegotiateFreelyAsClient,
		}
	}

	if dialContextfunc == nil {
		dialContextfunc = (&net.Dialer{
			Timeout: 30 * time.Second,
		}).DialContext
	}

	poolConfig := defaultTransportConfig
	if len(transportConfig) > 0 {
		poolConfig = transportConfig[0].withDefaults()
	}
	return &http.Client{
		Timeout: time.Second * 30,
		Transport: &http.Transport{
			TLSClientConfig:     tlsConfig,
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         dialContextfunc,
			TLSHandshakeTimeout: receivers.TLSHandshakeTimeout,
			MaxIdleConns:        poolConfig.MaxIdleConns,
			MaxIdleConnsPerHost: poolConfig.MaxIdleConnsPerHost,
			MaxConnsPerHost:     poolConfig.MaxConnsPerHost,
			IdleConnTimeout:     poolConfig.IdleConnTimeout,
		},
	}
}
