package v1

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestConfigJSONEncoding(t *testing.T) {
	for _, duration := range []any{nil, "", "0s", "1m"} {
		for _, apiURL := range []string{"https://example.com/rest/api/3", "/relative/path", "ftp://example.com"} {
			input := map[string]any{
				"api_url": apiURL, "reopen_duration": duration,
				"project": "test", "issue_type": "Bug", "api_token": "token",
				"labels": []string{"one", "two"},
				"fields": map[string]any{"nested": map[string]any{"value": float64(2)}, "text": "123", "bool": true},
			}
			data, err := json.Marshal(input)
			require.NoError(t, err)
			var cfg Config
			require.NoError(t, json.Unmarshal(data, &cfg))
			require.Equal(t, apiURL, cfg.URL.String())
			wantDuration := model.Duration(0)
			if duration == "1m" {
				wantDuration = model.Duration(time.Minute)
			}
			require.Equal(t, wantDuration, cfg.ReopenDuration)
			require.Equal(t, "token", cfg.Token)
			require.Equal(t, input["fields"], cfg.Fields)
			data, err = json.Marshal(cfg)
			require.NoError(t, err)
			var roundTrip Config
			require.NoError(t, json.Unmarshal(data, &roundTrip))
			require.Equal(t, cfg, roundTrip)
		}
	}

	for _, input := range []string{`{}`, `{"api_url":null,"reopen_duration":null}`} {
		var cfg Config
		require.NoError(t, json.Unmarshal([]byte(input), &cfg))
		require.Zero(t, cfg)
	}

	for _, input := range []string{
		`{"api_url":"http://invalid^^^"}`,
		`{"api_url":123}`,
		`{"reopen_duration":"invalid"}`,
		`{"reopen_duration":123}`,
	} {
		cfg := Config{Project: "unchanged"}
		require.Error(t, json.Unmarshal([]byte(input), &cfg))
		require.Equal(t, Config{Project: "unchanged"}, cfg)
	}
}

func TestConfigYAMLEncoding(t *testing.T) {
	for _, duration := range []string{"null", `""`, "0s", "1m"} {
		var cfg Config
		require.NoError(t, yaml.Unmarshal([]byte("api_url: /relative/path\nreopen_duration: "+duration+"\nfields:\n  nested:\n    value: 2\n  text: '123'\n"), &cfg))
		require.Equal(t, &url.URL{Path: "/relative/path"}, cfg.URL)
		wantDuration := model.Duration(0)
		if duration == "1m" {
			wantDuration = model.Duration(time.Minute)
		}
		require.Equal(t, wantDuration, cfg.ReopenDuration)
		require.Equal(t, map[string]any{"nested": map[string]any{"value": float64(2)}, "text": "123"}, cfg.Fields)
		data, err := yaml.Marshal(cfg)
		require.NoError(t, err)
		var roundTrip Config
		require.NoError(t, yaml.Unmarshal(data, &roundTrip))
		require.Equal(t, cfg, roundTrip)
	}
	for _, input := range []string{"api_url: http://invalid^^^", "api_url: 123", "reopen_duration: invalid", "reopen_duration: 123"} {
		cfg := Config{Project: "unchanged"}
		require.Error(t, yaml.Unmarshal([]byte(input), &cfg))
		require.Equal(t, Config{Project: "unchanged"}, cfg)
	}
}

func TestNewConfigOptionalDuration(t *testing.T) {
	for _, value := range []string{`null`, `""`, `"0s"`} {
		cfg, err := NewConfig(json.RawMessage(`{"api_url":"/relative/path","project":"test","issue_type":"Bug","api_token":"token","reopen_duration":`+value+`}`), receiversTesting.DecryptForTesting(nil))
		require.NoError(t, err)
		require.Equal(t, &url.URL{Path: "/relative/path"}, cfg.URL)
		require.Zero(t, cfg.ReopenDuration)
	}
}
