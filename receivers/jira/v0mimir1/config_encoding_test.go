package v0mimir1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	yamlv2 "gopkg.in/yaml.v2"
	yamlv3 "gopkg.in/yaml.v3"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestJiraFieldConfigJSONDecoding(t *testing.T) {
	base := `"api_url":"http://localhost","project":"PROJ","issue_type":"Bug"`
	for _, tc := range []struct {
		name     string
		field    string
		expected JiraFieldConfig
	}{
		{"legacy scalar", `"summary":"x"`, JiraFieldConfig{Template: "x"}},
		{"object false", `"summary":{"template":"x","enable_update":false}`, JiraFieldConfig{Template: "x", EnableUpdate: new(false)}},
		{"object true", `"summary":{"template":"x","enable_update":true}`, JiraFieldConfig{Template: "x", EnableUpdate: new(true)}},
		{"object omitted update", `"summary":{"template":"x"}`, JiraFieldConfig{Template: "x"}},
		{"object null update", `"summary":{"template":"x","enable_update":null}`, JiraFieldConfig{Template: "x"}},
		{"null matches YAML zero value", `"summary":null`, JiraFieldConfig{}},
		{"omitted retains default", ``, DefaultConfig.Summary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sep := ""
			if tc.field != "" {
				sep = ","
			}
			cfg, err := NewConfig(json.RawMessage("{"+base+sep+tc.field+"}"), receiversTesting.DecryptForTesting(nil))
			require.NoError(t, err)
			require.Equal(t, tc.expected, cfg.Summary)
		})
	}
}

func TestJiraFieldConfigYAMLDecoding(t *testing.T) {
	for _, tc := range []struct {
		name       string
		field      string
		value      string
		expectedV2 JiraFieldConfig
		expectedV3 JiraFieldConfig
	}{
		{"legacy scalar summary", "summary", "x", JiraFieldConfig{Template: "x"}, JiraFieldConfig{Template: "x"}},
		{"legacy scalar description", "description", "x", JiraFieldConfig{Template: "x"}, JiraFieldConfig{Template: "x"}},
		{"object false", "summary", "{template: x, enable_update: false}", JiraFieldConfig{Template: "x", EnableUpdate: new(false)}, JiraFieldConfig{Template: "x", EnableUpdate: new(false)}},
		{"object true", "summary", "{template: x, enable_update: true}", JiraFieldConfig{Template: "x", EnableUpdate: new(true)}, JiraFieldConfig{Template: "x", EnableUpdate: new(true)}},
		{"object omitted update", "summary", "{template: x}", JiraFieldConfig{Template: "x"}, JiraFieldConfig{Template: "x"}},
		{"object null update", "summary", "{template: x, enable_update: null}", JiraFieldConfig{Template: "x"}, JiraFieldConfig{Template: "x"}},
		{"null", "summary", "null", JiraFieldConfig{}, DefaultConfig.Summary},
		{"omitted", "", "", DefaultConfig.Summary, DefaultConfig.Summary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := "api_url: http://localhost\nproject: PROJ\nissue_type: Bug\n"
			if tc.field != "" {
				raw += tc.field + ": " + tc.value + "\n"
			}
			var v2 Config
			require.NoError(t, yamlv2.Unmarshal([]byte(raw), &v2))
			if tc.field == "description" {
				require.Equal(t, tc.expectedV2, v2.Description)
			} else {
				require.Equal(t, tc.expectedV2, v2.Summary)
			}
			require.Equal(t, "auto", v2.APIType)

			var v3 Config
			require.NoError(t, yamlv3.Unmarshal([]byte(raw), &v3))
			if tc.field == "description" {
				require.Equal(t, tc.expectedV3, v3.Description)
			} else {
				require.Equal(t, tc.expectedV3, v3.Summary)
			}
			require.Equal(t, "auto", v3.APIType)
		})
	}
}

func TestJiraFieldConfigMarshalFormsAndRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field JiraFieldConfig
		json  string
		yaml  string
	}{
		{"object nil", JiraFieldConfig{Template: "x"}, `{"template":"x"}`, "template: x\n"},
		{"object false", JiraFieldConfig{Template: "x", EnableUpdate: new(false)}, `{"template":"x","enable_update":false}`, "template: x\nenable_update: false\n"},
		{"object true", JiraFieldConfig{Template: "x", EnableUpdate: new(true)}, `{"template":"x","enable_update":true}`, "template: x\nenable_update: true\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.field)
			require.NoError(t, err)
			require.JSONEq(t, tc.json, string(b))
			var decoded JiraFieldConfig
			require.NoError(t, json.Unmarshal(b, &decoded))
			require.Equal(t, tc.field, decoded)
			y, err := yamlv2.Marshal(tc.field)
			require.NoError(t, err)
			require.Equal(t, tc.yaml, string(y))
			decoded = JiraFieldConfig{}
			require.NoError(t, yamlv2.Unmarshal(y, &decoded))
			require.Equal(t, tc.field, decoded)
		})
	}
}

func TestJiraAPITypeValidationJSONAndYAML(t *testing.T) {
	for _, apiType := range []string{"auto", "cloud", "datacenter"} {
		t.Run(apiType, func(t *testing.T) {
			_, err := NewConfig(json.RawMessage(`{"api_url":"http://localhost","project":"PROJ","issue_type":"Bug","api_type":"`+apiType+`"}`), receiversTesting.DecryptForTesting(nil))
			require.NoError(t, err)
			var cfg Config
			require.NoError(t, yamlv2.Unmarshal([]byte("project: PROJ\nissue_type: Bug\napi_type: "+apiType+"\n"), &cfg))
		})
	}
	for _, apiType := range []string{"", "invalid"} {
		t.Run("reject "+apiType, func(t *testing.T) {
			_, err := NewConfig(json.RawMessage(`{"api_url":"http://localhost","project":"PROJ","issue_type":"Bug","api_type":"`+apiType+`"}`), receiversTesting.DecryptForTesting(nil))
			require.EqualError(t, err, "unknown api_type on jira_config, must be auto, cloud or datacenter")
			var cfg Config
			err = yamlv2.Unmarshal([]byte("project: PROJ\nissue_type: Bug\napi_type: '"+apiType+"'\n"), &cfg)
			require.EqualError(t, err, "unknown api_type on jira_config, must be auto, cloud or datacenter")
		})
	}
	cfg, err := NewConfig(json.RawMessage(`{"api_url":"http://localhost","project":"PROJ","issue_type":"Bug"}`), receiversTesting.DecryptForTesting(nil))
	require.NoError(t, err)
	require.Equal(t, "auto", cfg.APIType)

	var yamlCfg Config
	require.NoError(t, yamlv2.Unmarshal([]byte("project: PROJ\nissue_type: Bug\n"), &yamlCfg))
	require.Equal(t, "auto", yamlCfg.APIType)
}

func TestLegacyStoredConfigRemarshal(t *testing.T) {
	legacy := `{"api_url":"http://localhost","project":"PROJ","issue_type":"Bug","summary":"test summary","description":"test description","priority":"High","labels":["alertmanager"],"custom_fields":{"customfield_10000":"test customfield_10000"},"send_resolved":true}`
	cfg, err := NewConfig(json.RawMessage(legacy), receiversTesting.DecryptForTesting(nil))
	require.NoError(t, err)
	cfg.HTTPConfig = nil
	b, err := json.Marshal(cfg)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(b, &got))
	require.Equal(t, map[string]any{"template": "test summary"}, got["summary"])
	require.Equal(t, map[string]any{"template": "test description"}, got["description"])

	var roundTripped Config
	require.NoError(t, json.Unmarshal(b, &roundTripped))
	require.Equal(t, cfg, roundTripped)
}
