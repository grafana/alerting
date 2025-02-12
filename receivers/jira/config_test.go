package jira

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestNewConfig(t *testing.T) {
	testURL, err := url.Parse("http://localhost")
	require.NoError(t, err)

	cases := []struct {
		name              string
		settings          string
		secureSettings    map[string][]byte
		expectedConfig    Config
		expectedInitError string
	}{
		{
			name:              "Error if empty",
			settings:          "",
			expectedInitError: `failed to unmarshal settings`,
		},
		{
			name:              "Error if empty JSON object",
			settings:          `{}`,
			expectedInitError: `could not find api_url property in settings`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"api_url": "http://localhost", "project": "test", "issue_type": "test"}`,
			secureSettings: map[string][]byte{
				"api_token": []byte("test-token"),
			},
			expectedConfig: Config{
				URL:         testURL,
				Project:     "test",
				IssueType:   "test",
				Summary:     `{{ template "jira.default.summary" . }}`,
				Description: `{{ template "jira.default.description" . }}`,
				Priority:    `{{ template "jira.default.priority" . }}`,
				Token:       "test-token",
			},
		},
		{
			name: "Extracts all fields",
			settings: `{
				"api_url":           	"http://localhost",
				"project":           	"Test Project",
				"summary":           	"Test Summary",
				"description":       	"Test Description",
				"labels":            	["Test Label"],
				"priority":          	"Test Priority",
				"issue_type":         	"Test Issue Type",
				"reopen_transition":  	"Test Reopen Transition",
				"resolve_transition": 	"Test Resolve Transition",
				"wont_fix_resolution":	"Test Won't Fix Resolution",
				"reopen_duration":    	"1m",
				"fields":            	{"test-field": "test-value"}
			}`,
			secureSettings: map[string][]byte{
				"password": []byte("test-password"),
				"user":     []byte("test-username"),
			},
			expectedConfig: Config{
				URL:               testURL,
				Project:           "Test Project",
				Summary:           "Test Summary",
				Description:       "Test Description",
				Labels:            []string{"Test Label"},
				Priority:          "Test Priority",
				IssueType:         "Test Issue Type",
				ReopenTransition:  "Test Reopen Transition",
				ResolveTransition: "Test Resolve Transition",
				WontFixResolution: "Test Won't Fix Resolution",
				ReopenDuration:    model.Duration(1 * time.Minute),
				Fields:            map[string]any{"test-field": "test-value"},
				User:              "test-username",
				Password:          "test-password",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(c.secureSettings))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
