package v1

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestMaxAlertsDecoding(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int
	}{
		{`12`, 12}, {`"12"`, 12}, {`null`, 0}, {`""`, 0},
		{`"invalid"`, 0}, {`1.5`, 0}, {`1e2`, 0}, {`true`, 0},
		{`[]`, 0}, {`{}`, 0}, {`" 12 "`, 0}, {`"12\n"`, 0},
		{`-12`, -12}, {`"+12"`, 12},
		{`"999999999999999999999999999999"`, math.MaxInt},
		{`"-999999999999999999999999999999"`, math.MinInt},
	} {
		t.Run(tc.input, func(t *testing.T) {
			cfg, err := NewConfig(json.RawMessage(`{"url":"http://localhost","maxAlerts":`+tc.input+`}`), receiversTesting.DecryptForTesting(nil))
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.MaxAlerts)
		})
	}
}

func TestConfigRoundTrip(t *testing.T) {
	cfg, err := NewConfig(json.RawMessage(FullValidConfigForTesting), receiversTesting.DecryptForTesting(nil))
	require.NoError(t, err)
	for _, codec := range []struct {
		name      string
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{
		{"JSON", json.Marshal, json.Unmarshal},
		{"YAML", yaml.Marshal, yaml.Unmarshal},
	} {
		t.Run(codec.name, func(t *testing.T) {
			data, err := codec.marshal(cfg)
			require.NoError(t, err)
			var got Config
			require.NoError(t, codec.unmarshal(data, &got))
			require.Equal(t, cfg, got)
		})
	}
}

func TestConfigYAMLMaxAlerts(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int
	}{
		{"12", 12}, {`"12"`, 12}, {"null", 0}, {`""`, 0},
		{"invalid", 0}, {"1.5", 0}, {"true", 0}, {"-12", -12},
		{"1e2", 0}, {"0x10", 0}, {"[]", 0}, {"{}", 0},
	} {
		var cfg Config
		require.NoError(t, yaml.Unmarshal([]byte("url: http://localhost\nmaxAlerts: "+tc.input), &cfg))
		require.Equal(t, tc.want, cfg.MaxAlerts)
	}
}

func TestConfigYAMLNativeScalars(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte("url: 123\ntitle: true\nmaxAlerts: 1e2"), &cfg))
	require.Equal(t, "123", cfg.URL)
	require.Equal(t, "true", cfg.Title)
	require.Zero(t, cfg.MaxAlerts)

	for _, input := range []string{
		"defaults: &defaults {maxAlerts: '12'}\n<<: *defaults\nurl: http://localhost",
		"count: &count '12'\nmaxAlerts: *count\nurl: http://localhost",
	} {
		require.NoError(t, yaml.Unmarshal([]byte(input), &cfg))
		require.Equal(t, 12, cfg.MaxAlerts)
	}
	require.Error(t, yaml.Unmarshal([]byte("maxAlerts: 1\nmaxAlerts: 2"), &cfg))
}

func TestConfigErrorResults(t *testing.T) {
	decrypt := receiversTesting.DecryptForTesting(nil)
	for _, input := range []string{`{"title":"ignored"}`, `{"url":"http://localhost","httpMethod":123}`} {
		cfg, err := NewConfig(json.RawMessage(input), decrypt)
		require.Error(t, err)
		require.Zero(t, cfg)
	}
	cfg, err := NewConfig(json.RawMessage(`{"url":"http://localhost","username":"user","password":"password","authorization_credentials":"token","title":"not copied","message":"not copied"}`), decrypt)
	require.ErrorContains(t, err, "both HTTP Basic Authentication and Authorization Header")
	require.Equal(t, Config{
		URL: "http://localhost", HTTPMethod: "POST", User: "user", Password: "password",
		AuthorizationScheme: "Bearer", AuthorizationCredentials: "token",
	}, cfg)
}
