package http

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_NewTLSClient(t *testing.T) {
	tc := []struct {
		name   string
		cfg    *tls.Config
		expCfg *tls.Config
	}{
		{
			name:   "empty TLSConfig",
			expCfg: &tls.Config{Renegotiation: tls.RenegotiateFreelyAsClient},
		},
		{
			name:   "valid TLSConfig",
			cfg:    &tls.Config{InsecureSkipVerify: true},
			expCfg: &tls.Config{InsecureSkipVerify: true},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			c := NewTLSClient(tt.cfg, nil)
			require.Equal(t, tt.expCfg, c.Transport.(*http.Transport).TLSClientConfig)
			transport := c.Transport.(*http.Transport)
			require.Equal(t, 25, transport.MaxIdleConnsPerHost)
			require.Equal(t, 25, transport.MaxConnsPerHost)
			require.Equal(t, 90*time.Second, transport.IdleConnTimeout)
			require.False(t, transport.DisableKeepAlives)
		})
	}
}
