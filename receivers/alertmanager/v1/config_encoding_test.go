package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestConfigEncoding(t *testing.T) {
	for _, rawURL := range []string{
		"http://localhost", "http://localhost/", "http://localhost//",
		"/", "/relative", "http://localhost?query=value", "http://localhost#fragment",
		"http://localhost/a%2Fb", " http://one/ , , http://two/prefix ",
		"http://localhost/api/v2/alerts",
	} {
		t.Run(rawURL, func(t *testing.T) {
			input, err := json.Marshal(map[string]string{"url": rawURL, "basicAuthUser": "user", "basicAuthPassword": "password"})
			require.NoError(t, err)
			cfg, err := NewConfig(input, receiversTesting.DecryptForTesting(nil))
			require.NoError(t, err)
			for _, codec := range []struct {
				marshal   func(any) ([]byte, error)
				unmarshal func([]byte, any) error
			}{
				{json.Marshal, json.Unmarshal}, {yaml.Marshal, yaml.Unmarshal},
			} {
				data, err := codec.marshal(cfg)
				require.NoError(t, err)
				var roundTrip Config
				require.NoError(t, codec.unmarshal(data, &roundTrip))
				require.Equal(t, cfg, roundTrip)
			}
			encoded, err := json.Marshal(cfg)
			require.NoError(t, err)
			roundTrip, err := NewConfig(encoded, receiversTesting.DecryptForTesting(nil))
			require.NoError(t, err)
			require.Equal(t, cfg, roundTrip)
		})
	}
}

func TestConfigURLCompatibility(t *testing.T) {
	for _, suffix := range []string{`""`, `null`} {
		cfg, err := NewConfig(json.RawMessage(`{"url":"http://localhost","url":`+suffix+`}`), receiversTesting.DecryptForTesting(nil))
		require.NoError(t, err)
		require.Equal(t, "http://localhost/api/v2/alerts", cfg.URLs[0].String())
	}
	for _, input := range []string{`{}`, `null`, `{"url":null}`, `{"url":" , , "}`, `{"url":"http://localhost","url":" , "}`} {
		cfg, err := NewConfig(json.RawMessage(input), receiversTesting.DecryptForTesting(nil))
		require.EqualError(t, err, "could not find url property in settings")
		require.Zero(t, cfg)
	}
	for _, input := range []string{`{"url":123}`, `{"url":[]}`, `{"url":true}`, `{"url":{}}`} {
		cfg, err := NewConfig(json.RawMessage(input), receiversTesting.DecryptForTesting(nil))
		require.ErrorContains(t, err, "failed to unmarshal settings")
		require.Zero(t, cfg)
	}
}

func TestConfigDecryptionOrder(t *testing.T) {
	var calls []string
	decrypt := func(key, fallback string) (string, bool) {
		calls = append(calls, key)
		return "decrypted", true
	}
	cfg, err := NewConfig(json.RawMessage(`{"url":"://bad"}`), decrypt)
	require.ErrorContains(t, err, "invalid url property")
	require.Zero(t, cfg)
	require.Empty(t, calls)
	cfg, err = NewConfig(json.RawMessage(`{"url":"http://localhost","basicAuthUser":"user"}`), decrypt)
	require.NoError(t, err)
	require.Equal(t, []string{"basicAuthPassword"}, calls)
	require.Equal(t, "decrypted", cfg.Password)
	require.Equal(t, "user", cfg.User)
}
