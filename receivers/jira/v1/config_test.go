package v1

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
			name:              "Error if URL is empty",
			settings:          `{ "api_url": "" }`,
			expectedInitError: `could not find api_url property in settings`,
		},
		{
			name:              "Error if URL is not valid",
			settings:          `{ "api_url": "http://invalid-url^^^" }`,
			expectedInitError: `field api_url is not a valid URL`,
		},
		{
			name:              "Error if missing project",
			settings:          `{"api_url": "http://localhost", "issue_type": "test"}`,
			expectedInitError: `missing project in jira_config`,
		},
		{
			name:              "Error if missing issue_type",
			settings:          `{"api_url": "http://localhost", "project": "test"}`,
			expectedInitError: `missing issue_type in jira_config`,
		},
		{
			name: "Error if reopen_duration is invalid",
			settings: `{
				"api_url": "http://localhost", 
				"project": "test", 
				"issue_type": "test",
				"reopen_duration": "10"
			}`,
			expectedInitError: `field reopen_duration is not a valid duration`,
		},
		{
			name:     "Error if dedup_key_field is invalid",
			settings: `{"api_url": "http://localhost", "project": "test", "issue_type": "test", "dedup_key_field": "invalid_key"}`,
			secureSettings: map[string][]byte{
				"api_token": []byte("test-token"),
			},
			expectedInitError: `dedup_key_field must match the format [0-9]+`,
		},
		{
			name:     "Error if both user/password and token provided",
			settings: `{"api_url": "http://localhost", "project": "test", "issue_type": "test"}`,
			secureSettings: map[string][]byte{
				"user":      []byte("test-user"),
				"password":  []byte("test-password"),
				"api_token": []byte("test-token"),
			},
			expectedInitError: `provided both token and user/password, only one is allowed at a time`,
		},
		{
			name:              "Error if neither user/password nor token provided",
			settings:          `{"api_url": "http://localhost", "project": "test", "issue_type": "test"}`,
			expectedInitError: `either token or both user and password must be set`,
		},
		{
			name:     "Minimal valid configuration (api_token)",
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
			name:     "Minimal valid configuration (user/password)",
			settings: `{"api_url": "http://localhost", "project": "test", "issue_type": "test"}`,
			secureSettings: map[string][]byte{
				"user":     []byte("test-user"),
				"password": []byte("test-password"),
			},
			expectedConfig: Config{
				URL:         testURL,
				Project:     "test",
				IssueType:   "test",
				Summary:     `{{ template "jira.default.summary" . }}`,
				Description: `{{ template "jira.default.description" . }}`,
				Priority:    `{{ template "jira.default.priority" . }}`,
				User:        "test-user",
				Password:    "test-password",
			},
		},
		{
			name: "should parse string fields if valid json",
			settings: `{
				"api_url": "http://localhost", 
				"project": "test", 
				"issue_type": "test",
				"fields": {
					"customfield_10001": {"value": "green", "child": {"value":"blue"} },
					"customfield_10002": "2011-10-03",
					"customfield_10004": "Free text goes here.  Type away!",
					"customfield_10005": "{ \"name\": \"jira-developers\" }",
					"customfield_10008": [ {"value": "red" }, {"value": "blue" }, {"value": "green" }],
					"customfield_10009": "[{\"name\":\"charlie\"},{\"name\":\"bjones\"},{\"name\":\"tdurden\"}]",
					"customfield_10010": 42.07
				}
			}`,
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
				Fields: map[string]any{
					"customfield_10001": map[string]any{
						"value": "green",
						"child": map[string]any{
							"value": "blue",
						},
					},
					"customfield_10002": "2011-10-03",
					"customfield_10004": "Free text goes here.  Type away!",
					"customfield_10005": map[string]any{
						"name": "jira-developers",
					},
					"customfield_10008": []any{
						map[string]any{
							"value": "red",
						},
						map[string]any{
							"value": "blue",
						},
						map[string]any{
							"value": "green",
						},
					},
					"customfield_10009": []any{
						map[string]any{
							"name": "charlie",
						},
						map[string]any{
							"name": "bjones",
						},
						map[string]any{
							"name": "tdurden",
						},
					},
					"customfield_10010": 42.07,
				},
			},
		},
		{
			name:     "Extracts all fields no secrets",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				URL:               testURL,
				Project:           "Test Project",
				Summary:           "Test Summary",
				Description:       "Test Description",
				Labels:            []string{"Test Label", "Test Label 2"},
				Priority:          "Test Priority",
				IssueType:         "Test Issue Type",
				ReopenTransition:  "Test Reopen Transition",
				ResolveTransition: "Test Resolve Transition",
				WontFixResolution: "Test Won't Fix Resolution",
				ReopenDuration:    model.Duration(1 * time.Minute),
				DedupKeyFieldName: "10000",
				Fields: map[string]any{
					"test-field": "test-value",
				},
				User:     "user",
				Password: "password",
			},
		},
		{
			name:           "Extracts all fields + override from secrets",
			settings:       FullValidConfigForTesting,
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				URL:               testURL,
				Project:           "Test Project",
				Summary:           "Test Summary",
				Description:       "Test Description",
				Labels:            []string{"Test Label", "Test Label 2"},
				Priority:          "Test Priority",
				IssueType:         "Test Issue Type",
				ReopenTransition:  "Test Reopen Transition",
				ResolveTransition: "Test Resolve Transition",
				WontFixResolution: "Test Won't Fix Resolution",
				ReopenDuration:    model.Duration(1 * time.Minute),
				DedupKeyFieldName: "10000",
				Fields: map[string]any{
					"test-field": "test-value",
				},
				User:     "test-user",
				Password: "test-password",
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
