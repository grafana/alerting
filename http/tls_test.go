package http

import (
	"crypto/tls"
	"net/http"
	"testing"

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
			c := NewTLSClient(tt.cfg)
			require.Equal(t, tt.expCfg, c.Transport.(*http.Transport).TLSClientConfig)
		})
	}
}
