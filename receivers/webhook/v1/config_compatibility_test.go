package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

func TestPayloadPointerCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields string
		want   CustomPayload
	}{
		{"null", `"payload":null`, CustomPayload{}},
		{"empty", `"payload":{}`, CustomPayload{}},
		{"object then null", `"payload":{"template":"custom","vars":{"old":"value"}},"payload":null`, CustomPayload{}},
		{"object null object", `"payload":{"template":"custom","vars":{"old":"value"}},"payload":null,"payload":{"vars":{"new":"value"}}`, CustomPayload{Vars: map[string]string{"new": "value"}}},
		{"objects merge", `"payload":{"template":"custom"},"payload":{"vars":{"new":"value"}}`, CustomPayload{Template: "custom", Vars: map[string]string{"new": "value"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := NewConfig(json.RawMessage(`{"url":"http://localhost",`+tc.fields+`}`), receiversTesting.DecryptForTesting(nil))
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.Payload)
		})
	}
}

func TestOptionalSecurityConfigCompatibility(t *testing.T) {
	for _, value := range []string{`null`, `{}`, `{"ServerName":"ignored"}`} {
		t.Run(value, func(t *testing.T) {
			var calls []string
			decrypt := func(key, fallback string) (string, bool) {
				calls = append(calls, key)
				switch key {
				case "tlsConfig.caCertificate", "tlsConfig.clientCertificate", "tlsConfig.clientKey", "hmacConfig.secret":
					return "decrypted:" + key, true
				default:
					return fallback, false
				}
			}
			cfg, err := NewConfig(json.RawMessage(`{"url":"http://localhost","tlsConfig":`+value+`,"hmacConfig":`+value+`}`), decrypt)
			require.NoError(t, err)
			if value == "null" {
				require.Nil(t, cfg.TLSConfig)
				require.Nil(t, cfg.HMACConfig)
				require.Equal(t, []string{"username", "password", "authorization_credentials"}, calls)
				return
			}
			require.Equal(t, &receivers.TLSConfig{
				CACertificate:     "decrypted:tlsConfig.caCertificate",
				ClientCertificate: "decrypted:tlsConfig.clientCertificate",
				ClientKey:         "decrypted:tlsConfig.clientKey",
			}, cfg.TLSConfig)
			require.Equal(t, &receivers.HMACConfig{Secret: "decrypted:hmacConfig.secret"}, cfg.HMACConfig)
			require.Equal(t, []string{"username", "password", "authorization_credentials", "tlsConfig.caCertificate", "tlsConfig.clientCertificate", "tlsConfig.clientKey", "hmacConfig.secret"}, calls)
		})
	}
}

func TestRestrictedHeadersErrorResult(t *testing.T) {
	cfg, err := NewConfig(json.RawMessage(`{"url":"http://localhost","headers":{"Authorization":"blocked","X-Test":"allowed"},"payload":{"template":"custom"},"tlsConfig":{"ServerName":"ignored"},"hmacConfig":{"secret":"secret"}}`), receiversTesting.DecryptForTesting(nil))
	require.EqualError(t, err, `custom headers ["Authorization"] are not allowed`)
	require.Equal(t, Config{
		URL: "http://localhost", HTTPMethod: "POST",
		Title: templates.DefaultMessageTitleEmbed, Message: templates.DefaultMessageEmbed,
		Payload:   CustomPayload{Template: "custom"},
		TLSConfig: &receivers.TLSConfig{}, HMACConfig: &receivers.HMACConfig{Secret: "secret"},
	}, cfg)
}
