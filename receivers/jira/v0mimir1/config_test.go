package v0mimir1

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	httpcfg "github.com/grafana/alerting/http/v0mimir"
	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestJiraConfiguration(t *testing.T) {
	tc := []struct {
		name     string
		in       string
		expected error
		assert   func(t *testing.T, cfg Config)
	}{
		{
			name: "with missing project - it fails",
			in: `
issue_type: Bug
`,
			expected: errors.New("missing project in jira_config"),
		},
		{
			name: "with missing issue_type - it fails",
			in: `
project: OPS
`,
			expected: errors.New("missing issue_type in jira_config"),
		},
		{
			name: "with project and issue_type set - it succeeds",
			in: `
project: OPS
issue_type: Bug
`,
		},
		{
			name: "with fields using yaml tag - it succeeds",
			in: `
project: OPS
issue_type: Bug
fields:
  customfield_10001: value
`,
			assert: func(t *testing.T, cfg Config) {
				require.Equal(t, map[string]any{"customfield_10001": "value"}, cfg.Fields)
			},
		},
		{
			name: "json tags are supported - custom_fields fallback",
			in: `
project: OPS
issue_type: Bug
custom_fields:
  customfield_10001: value
`,
			assert: func(t *testing.T, cfg Config) {
				require.Equal(t, map[string]any{"customfield_10001": "value"}, cfg.Fields)
			},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			err := yaml.UnmarshalStrict([]byte(tt.in), &cfg)

			require.Equal(t, tt.expected, err)
			if tt.assert != nil && err == nil {
				tt.assert(t, cfg)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("FullValidConfigForTesting is valid", func(t *testing.T) {
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(FullValidConfigForTesting), &cfg)
		require.NoError(t, err)
		require.NoError(t, cfg.Validate())
	})
	cases := []struct {
		name        string
		mutate      func(cfg *Config)
		expectedErr string
	}{
		{
			name:   "GetFullValidConfig is valid",
			mutate: func(cfg *Config) {},
		},
		{
			name:        "Missing project",
			mutate:      func(cfg *Config) { cfg.Project = "" },
			expectedErr: "missing project in jira_config",
		},
		{
			name:        "Missing issue_type",
			mutate:      func(cfg *Config) { cfg.IssueType = "" },
			expectedErr: "missing issue_type in jira_config",
		},
		{
			name:        "Missing api_url",
			mutate:      func(cfg *Config) { cfg.APIURL = nil },
			expectedErr: "missing api_url in jira_config",
		},
		{
			name: "Invalid http_config",
			mutate: func(cfg *Config) {
				cfg.HTTPConfig = &httpcfg.HTTPClientConfig{
					BasicAuth: &httpcfg.BasicAuth{},
					OAuth2:    &httpcfg.OAuth2{},
				}
			},
			expectedErr: "invalid http_config",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := GetFullValidConfig()
			c.mutate(&cfg)
			err := cfg.Validate()

			if c.expectedErr != "" {
				require.ErrorContains(t, err, c.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNewConfig(t *testing.T) {
	defaultHTTPConfig := httpcfg.DefaultHTTPClientConfig

	cases := []struct {
		name              string
		settings          string
		secrets           map[string][]byte
		expectedConfig    Config
		expectedInitError string
	}{
		{
			name:              "Error if empty",
			settings:          "",
			expectedInitError: "failed to unmarshal settings",
		},
		{
			name:              "Error if missing project",
			settings:          `{"api_url": "http://localhost", "issue_type": "Bug"}`,
			expectedInitError: "missing project in jira_config",
		},
		{
			name:              "Error if missing issue_type",
			settings:          `{"api_url": "http://localhost", "project": "PROJ"}`,
			expectedInitError: "missing issue_type in jira_config",
		},
		{
			name: "Error if missing api_url",
			settings: `{
				"project": "PROJ",
				"issue_type": "Bug"
			}`,
			expectedInitError: "missing api_url in jira_config",
		},
		{
			name: "Minimal valid configuration",
			settings: `{
				"api_url": "http://localhost",
				"project": "PROJ",
				"issue_type": "Bug"
			}`,
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				APIURL:         receivers.MustParseURL("http://localhost"),
				Project:        "PROJ",
				IssueType:      "Bug",
				Summary:        DefaultConfig.Summary,
				Description:    DefaultConfig.Description,
				Priority:       DefaultConfig.Priority,
			},
		},
		{
			name: "Custom fields via JSON tag",
			settings: `{
				"api_url": "http://localhost",
				"project": "PROJ",
				"issue_type": "Bug",
				"custom_fields": {"customfield_10000": "value"}
			}`,
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: true},
				HTTPConfig:     &defaultHTTPConfig,
				APIURL:         receivers.MustParseURL("http://localhost"),
				Project:        "PROJ",
				IssueType:      "Bug",
				Summary:        DefaultConfig.Summary,
				Description:    DefaultConfig.Description,
				Priority:       DefaultConfig.Priority,
				Fields:         map[string]any{"customfield_10000": "value"},
			},
		},
		{
			name:     "FullValidConfigForTesting is valid",
			settings: FullValidConfigForTesting,
			expectedConfig: func() Config {
				cfg := DefaultConfig
				_ = json.Unmarshal([]byte(FullValidConfigForTesting), &cfg)
				httpCfg := httpcfg.DefaultHTTPClientConfig
				cfg.HTTPConfig = &httpCfg
				return cfg
			}(),
		},
		{
			name:     "GetFullValidConfig round-trips through JSON",
			settings: func() string { b, _ := json.Marshal(GetFullValidConfig()); return string(b) }(),
			expectedConfig: func() Config {
				cfg := GetFullValidConfig()
				cfg.HTTPConfig = &defaultHTTPConfig
				return cfg
			}(),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(c.secrets))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
