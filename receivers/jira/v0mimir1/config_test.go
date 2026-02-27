package v0mimir1

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"gopkg.in/yaml.v2"
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
