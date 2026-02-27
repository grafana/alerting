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
	"testing"

	commoncfg "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestHTTPHeadersJSONRoundTrip documents that Headers has a custom MarshalJSON
// that inlines the map but no UnmarshalJSON, so JSON→JSON round-trip silently
// drops all header data.
func TestHTTPHeadersJSONRoundTrip(t *testing.T) {
	original := HTTPClientConfig{
		FollowRedirects: true,
		EnableHTTP2:     true,
		HTTPHeaders: &Headers{
			Headers: map[string]Header{
				"X-Custom-Header": {Values: []string{"val1", "val2"}},
				"X-Secret-Header": {Secrets: []commoncfg.Secret{"secret"}},
			},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got HTTPClientConfig
	require.NoError(t, json.Unmarshal(data, &got))

	// FAILS: Headers.MarshalJSON inlines the map, but there is no UnmarshalJSON.
	// The default decoder finds no struct field matching "X-Custom-Header", so
	// HTTPHeaders.Headers is empty after the round-trip.
	require.Equal(t, original.HTTPHeaders.Headers, got.HTTPHeaders.Headers)
}

// TestOAuth2TLSConfigJSONKey documents that OAuth2.TLSConfig is missing a json
// tag, so the JSON encoder uses the Go field name "TLSConfig" (PascalCase)
// instead of the snake_case "tls_config" used by every other field.
func TestOAuth2TLSConfigJSONKey(t *testing.T) {
	cfg := OAuth2{
		ClientID: "id",
		TokenURL: "http://example.com/token",
		TLSConfig: TLSConfig{
			CAFile: "ca.pem",
		},
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	// FAILS: the actual key is "TLSConfig" because OAuth2.TLSConfig has no json tag.
	require.Contains(t, string(data), `"tls_config"`)
}

// TestOAuth2TLSConfigStructToJSONToYAML documents that because OAuth2.TLSConfig
// uses the wrong JSON key ("TLSConfig"), a struct serialized to JSON and then
// deserialized by a YAML parser loses the TLS configuration entirely.
func TestOAuth2TLSConfigStructToJSONToYAML(t *testing.T) {
	original := HTTPClientConfig{
		FollowRedirects: true,
		EnableHTTP2:     true,
		OAuth2: &OAuth2{
			ClientID: "id",
			TokenURL: "http://example.com/token",
			TLSConfig: TLSConfig{
				CAFile:   "ca.pem",
				CertFile: "cert.pem",
				KeyFile:  "key.pem",
			},
		},
	}

	jsonData, err := json.Marshal(original)
	require.NoError(t, err)

	var got HTTPClientConfig
	require.NoError(t, yaml.Unmarshal(jsonData, &got))

	// FAILS: json.Marshal produces key "TLSConfig" but the yaml tag is
	// "tls_config"; YAML sees no matching field and silently ignores it.
	require.Equal(t, original.OAuth2.TLSConfig, got.OAuth2.TLSConfig)
}

// TestOAuth2TLSConfigJSONSnakeCaseInput documents that external consumers who
// supply JSON with the conventional snake_case key "tls_config" for OAuth2 get
// silently ignored, because the field has no json tag.
func TestOAuth2TLSConfigJSONSnakeCaseInput(t *testing.T) {
	input := `{
		"client_id": "id",
		"token_url": "http://example.com/token",
		"tls_config": {"ca_file": "ca.pem"}
	}`

	var cfg OAuth2
	require.NoError(t, json.Unmarshal([]byte(input), &cfg))

	// FAILS: "tls_config" is silently ignored; only "TLSConfig" works.
	require.Equal(t, "ca.pem", cfg.TLSConfig.CAFile)
}
