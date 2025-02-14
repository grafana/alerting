package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestPrepareDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		apiPath     string
		expected    any
	}{
		{
			name:        "Valid JSON input for API v3",
			description: `{"key":"value"}`,
			apiPath:     "/3",
			expected:    map[string]any{"key": "value"},
		},
		{
			name:        "adfDocument document for non-json and API v3",
			description: `description`,
			apiPath:     "/3",
			expected:    simpleAdfDocument("description"),
		},
		{
			name:        "Long description API v3 with truncation to ADF",
			description: strings.Repeat("A", MaxDescriptionLenRunes+1),
			apiPath:     "/3",
			expected:    simpleAdfDocument(strings.Repeat("A", MaxDescriptionLenRunes-adfDocOverhead-1) + "…"),
		},
		{
			name:        "Fallback to string for invalid JSON (v3)",
			description: `{"key:"value",}`, // Invalid JSON
			apiPath:     "/3",
			expected:    simpleAdfDocument(`{"key:"value",}`),
		},
		{
			name:        "Plain string for default API (non-v3)",
			description: "This is a test description",
			apiPath:     "/2",
			expected:    "This is a test description",
		},
		{
			name:        "Truncated string for overly long description (non-v3)",
			description: strings.Repeat("B", MaxDescriptionLenRunes+10),
			apiPath:     "/2",
			expected:    strings.Repeat("B", MaxDescriptionLenRunes-1) + "…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			conf := Config{URL: &url.URL{Path: tt.apiPath}}
			logger := logging.FakeLogger{}
			notifier := &Notifier{conf: conf}

			// Execute
			desc := notifier.prepareDescription(tt.description, logger)
			assert.EqualValues(t, tt.expected, desc)
		})
	}
}

func TestPrepareIssueRequestBody(t *testing.T) {

	v2 := &url.URL{Path: "/2"}
	v3 := &url.URL{Path: "/3"}
	tmpl := templates.ForTests(t)
	alerts := []*types.Alert{
		{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "severity": "critical"},
				Annotations: model.LabelSet{"ann1": "annv1", "type": "bug", "project": "PROJ"},
			},
		},
	}
	tests := []struct {
		name       string
		path       string
		conf       Config
		expPayload string
	}{
		{
			name: "Fields with template v2",
			conf: Config{
				URL:         v2,
				Project:     "{{ .CommonAnnotations.project }}",
				IssueType:   "{{ .CommonAnnotations.type }}",
				Summary:     DefaultSummary,
				Description: DefaultDescription,
				Priority:    DefaultPriority,
				Labels:      []string{"{{ .CommonLabels.alertname }}"},
			},
			expPayload: `{
	          "fields": {
	            "description": "# Alerts Firing:\n\nLabels:\n  - alertname = alert1\n  - severity = critical\n\nAnnotations:\n  - ann1 = annv1\n  - project = PROJ\n  - type = bug\n\nSource: \n\n",
	            "issuetype": {
	              "name": "bug"
	            },
	            "labels": [
                  "alert1",
	              "ALERT{test_group}"
	            ],
	            "priority": {
	              "name": "High"
	            },
	            "project": {
	              "key": "PROJ"
	            },
	            "summary": "[FIRING:1]  (alert1 critical)"
	          }
	        }`,
		},
		{
			name: "Fields with template v3",
			conf: Config{
				URL:         v3,
				Project:     "{{ .CommonAnnotations.project }}",
				IssueType:   "{{ .CommonAnnotations.type }}",
				Summary:     DefaultSummary,
				Description: DefaultDescription,
				Priority:    DefaultPriority,
				Labels:      []string{"{{ .CommonLabels.alertname }}"},
			},
			expPayload: `{
	          "fields": {
	            "description": {
	              "version": 1,
	              "type": "doc",
	              "content": [
	                {
	                  "type": "paragraph",
	                  "content": [
	                    {
	                      "type": "text",
	                      "text": "# Alerts Firing:\n\nLabels:\n  - alertname = alert1\n  - severity = critical\n\nAnnotations:\n  - ann1 = annv1\n  - project = PROJ\n  - type = bug\n\nSource: \n\n"
	                    }
	                  ]
	                }
	              ]
	            },
	            "issuetype": {
	              "name": "bug"
	            },
	            "labels": [
                  "alert1",
	              "ALERT{test_group}"
	            ],
	            "priority": {
	              "name": "High"
	            },
	            "project": {
	              "key": "PROJ"
	            },
	            "summary": "[FIRING:1]  (alert1 critical)"
	          }
	        }`,
		},
		{
			name: "Fallback to default templates if invalid",
			conf: Config{
				URL:         v2,
				Project:     "{{",
				IssueType:   "{{",
				Summary:     "{{",
				Description: "{{",
				Priority:    "{{",
				Labels:      []string{"{{"},
			},
			expPayload: `{
	          "fields": {
	            "description": "# Alerts Firing:\n\nLabels:\n  - alertname = alert1\n  - severity = critical\n\nAnnotations:\n  - ann1 = annv1\n  - project = PROJ\n  - type = bug\n\nSource: \n\n",
	            "labels": [
	              "ALERT{test_group}"
	            ],
	            "summary": "[FIRING:1]  (alert1 critical)"
	          }
	        }`,
		},
		{
			name: "summary should be truncated",
			conf: Config{
				URL:         v2,
				Project:     "{{ .CommonAnnotations.project }}",
				IssueType:   "{{ .CommonAnnotations.type }}",
				Summary:     strings.Repeat("A", MaxSummaryLenRunes+1),
				Description: DefaultDescription,
				Priority:    DefaultPriority,
			},
			expPayload: fmt.Sprintf(`{
	          "fields": {
	            "description": "# Alerts Firing:\n\nLabels:\n  - alertname = alert1\n  - severity = critical\n\nAnnotations:\n  - ann1 = annv1\n  - project = PROJ\n  - type = bug\n\nSource: \n\n",
	            "issuetype": {
	              "name": "bug"
	            },
	            "labels": [
	              "ALERT{test_group}"
	            ],
	            "priority": {
	              "name": "High"
	            },
	            "project": {
	              "key": "PROJ"
	            },
	            "summary": "%s…"
	          }
	        }`, strings.Repeat("A", MaxSummaryLenRunes-1)),
		},
		{
			name: "should append fields to custom fields",
			conf: Config{
				URL:         v2,
				Summary:     "sum",
				Description: "desc",
				Priority:    "high",
				Fields: map[string]any{
					"customfield_10001": map[string]any{"value": "green"},
					"customfield_10002": "2011-10-03",
					"customfield_10004": "Free text goes here.  Type away!",
					"customfield_10008": []any{map[string]any{"value": "red"}},
					"customfield_10010": 42.07,
				},
			},
			expPayload: `{
	          "fields": {
	            "customfield_10001": {
	              "value": "green"
	            },
	            "customfield_10002": "2011-10-03",
	            "customfield_10004": "Free text goes here.  Type away!",
	            "customfield_10008": [
	              {
	                "value": "red"
	              }
	            ],
	            "customfield_10010": 42.07,
	            "description": "desc",
	            "labels": [
	              "ALERT{test_group}"
	            ],
	            "priority": {
	              "name": "high"
	            },
	            "summary": "sum"
	          }
	        }`,
		},
		{
			name: "should add group key to custom fields if dedup key field is set",
			conf: Config{
				URL:               v2,
				Summary:           "sum",
				Description:       "desc",
				Priority:          "high",
				DedupKeyFieldName: "12345",
				Fields: map[string]any{
					"customfield_12345": "should-override",
				},
			},
			expPayload: `{
	          "fields": {
	            "customfield_12345": "test_group",
	            "description": "desc",
	            "labels": [],
	            "priority": {
	              "name": "high"
	            },
	            "summary": "sum"
	          }
	        }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := New(tt.conf, receivers.Metadata{}, tmpl, nil, logging.FakeLogger{})

			issue := n.prepareIssueRequestBody(context.Background(), logging.FakeLogger{}, "test_group", alerts...)

			d, err := json.MarshalIndent(issue, "", "  ")
			require.NoError(t, err)
			t.Log(string(d))
			require.JSONEq(t, tt.expPayload, string(d))
		})
	}
}

func TestGetSearchJql(t *testing.T) {
	groupKey := "group1"
	tests := []struct {
		name        string
		conf        Config
		firing      bool
		expectedJql string
	}{
		{
			name: "firing and default configuration",
			conf: Config{
				Project: "TEST",
			},
			firing:      true,
			expectedJql: `statusCategory != Done and labels = "ALERT{group1}" and project="TEST" order by status ASC,resolutiondate DESC`,
		},
		{
			name: "firing and configuration with wont-fix resolution",
			conf: Config{
				Project:           "TEST",
				WontFixResolution: "won't fix",
			},
			firing:      true,
			expectedJql: `resolution != "won't fix" and statusCategory != Done and labels = "ALERT{group1}" and project="TEST" order by status ASC,resolutiondate DESC`,
		},
		{
			name: "firing and reopen transition is set",
			conf: Config{
				Project:          "TEST",
				ReopenTransition: "test",
			},
			firing:      true,
			expectedJql: `labels = "ALERT{group1}" and project="TEST" order by status ASC,resolutiondate DESC`,
		},
		{
			name: "firing and custom dedup key field",
			conf: Config{
				Project:           "TEST",
				DedupKeyFieldName: "12345",
			},
			firing:      true,
			expectedJql: `statusCategory != Done and (labels = "ALERT{group1}" or cf[12345] ~ "group1") and project="TEST" order by status ASC,resolutiondate DESC`,
		},
		{
			name: "resolved and default configuration",
			conf: Config{
				Project: "TEST",
			},
			firing:      false,
			expectedJql: `labels = "ALERT{group1}" and project="TEST" order by status ASC,resolutiondate DESC`,
		},
		{
			name: "resolved and reopen duration specified",
			conf: Config{
				Project:        "TEST",
				ReopenDuration: model.Duration(30 * time.Minute),
			},
			firing:      false,
			expectedJql: `(resolutiondate is EMPTY OR resolutiondate >= -30m) and labels = "ALERT{group1}" and project="TEST" order by status ASC,resolutiondate DESC`,
		},
		{
			name: "resolved and configuration with wont-fix resolution",
			conf: Config{
				Project:           "TEST",
				WontFixResolution: "won't fix",
			},
			firing:      false,
			expectedJql: `resolution != "won't fix" and labels = "ALERT{group1}" and project="TEST" order by status ASC,resolutiondate DESC`,
		},
		{
			name: "resolved and custom dedup key field",
			conf: Config{
				Project:           "TEST",
				DedupKeyFieldName: "12345",
			},
			firing:      false,
			expectedJql: `(labels = "ALERT{group1}" or cf[12345] ~ "group1") and project="TEST" order by status ASC,resolutiondate DESC`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSearchJql(tt.conf, groupKey, tt.firing)
			assert.Containsf(t, result.Fields, "status", "query should always request status")
			assert.NotZero(t, result.MaxResults, "query should always request more than 0 results")
			require.Equal(t, tt.expectedJql, result.JQL)
		})
	}
}

func TestNotify(t *testing.T) {
	ctx := notify.WithGroupKey(context.Background(), "test_group")
	groupKey, _ := notify.ExtractGroupKey(ctx)
	tmpl := templates.ForTests(t)
	baseUrl := "https://jira.example.com/2"

	alert := &types.Alert{
		Alert: model.Alert{
			Labels: model.LabelSet{
				"alertname": "TestAlert",
				"severity":  "critical",
			},
			Annotations: model.LabelSet{
				"summary":     "Test alert summary",
				"description": "Test alert description",
			},
			StartsAt: time.Now(),
		},
	}

	t.Run("creates a new issue if no existing", func(t *testing.T) {
		mock := receivers.NewMockWebhookSender()
		mock.SendWebhookFunc = func(ctx context.Context, cmd *receivers.SendWebhookSettings) error {
			switch cmd.URL {
			case baseUrl + "/search":
				return cmd.Validation(mustMarshal(issueSearchResult{}), 200)
			case baseUrl + "/issue":
				return cmd.Validation(nil, 201)
			default:
				t.FailNow()
				return nil
			}
		}

		u, _ := url.Parse(baseUrl)
		cfg := Config{
			URL:         u,
			Summary:     "sum",
			Description: "desc",
			Priority:    "high",
			User:        "test",
			Password:    "test",
		}
		n := New(cfg, receivers.Metadata{}, tmpl, mock, logging.FakeLogger{})
		retry, err := n.Notify(ctx, alert)
		require.NoError(t, err)
		require.False(t, retry)
		require.Len(t, mock.Calls, 2)

		searchRequest := mock.Calls[0].Args[1].(*receivers.SendWebhookSettings)
		assert.Equal(t, cfg.User, searchRequest.User)
		assert.Equal(t, cfg.Password, searchRequest.Password)
		assert.JSONEq(t, string(mustMarshal(getSearchJql(cfg, groupKey.Hash(), true))), searchRequest.Body)
		assert.Equal(t, baseUrl+"/search", searchRequest.URL)
		assert.Equal(t, "POST", searchRequest.HTTPMethod)

		submitRequest := mock.Calls[1].Args[1].(*receivers.SendWebhookSettings)
		assert.Equal(t, cfg.User, submitRequest.User)
		assert.Equal(t, cfg.Password, submitRequest.Password)
		assert.JSONEq(t, string(mustMarshal(n.prepareIssueRequestBody(ctx, logging.FakeLogger{}, groupKey.Hash(), alert))), submitRequest.Body)
		assert.Equal(t, baseUrl+"/issue", submitRequest.URL)
		assert.Equal(t, "POST", submitRequest.HTTPMethod)
	})

	t.Run("updates existing issue if firing", func(t *testing.T) {
		mock := receivers.NewMockWebhookSender()
		issueKey := "TEST-1"
		mock.SendWebhookFunc = func(ctx context.Context, cmd *receivers.SendWebhookSettings) error {
			switch cmd.URL {
			case baseUrl + "/search":
				return cmd.Validation(mustMarshal(issueSearchResult{
					Total: 1,
					Issues: []issue{
						{
							Key: issueKey,
							Fields: &issueFields{
								Status: &issueStatus{
									StatusCategory: keyValue{
										Key: "blah",
									},
								},
							},
							Transition: nil,
						},
					},
				}), 200)
			case baseUrl + "/issue/" + issueKey:
				return cmd.Validation(nil, 201)
			default:
				t.Fatalf("unexpected url: %s", cmd.URL)
				return nil
			}
		}

		u, _ := url.Parse(baseUrl)
		cfg := Config{
			URL:         u,
			Summary:     "sum",
			Description: "desc",
			Priority:    "high",
			User:        "test",
			Password:    "test",
		}
		n := New(cfg, receivers.Metadata{}, tmpl, mock, logging.FakeLogger{})
		retry, err := n.Notify(ctx, alert)
		require.NoError(t, err)
		require.False(t, retry)
		assert.Len(t, mock.Calls, 2)

		submitRequest := mock.Calls[1].Args[1].(*receivers.SendWebhookSettings)
		assert.Equal(t, cfg.User, submitRequest.User)
		assert.Equal(t, cfg.Password, submitRequest.Password)
		assert.JSONEq(t, string(mustMarshal(n.prepareIssueRequestBody(ctx, logging.FakeLogger{}, groupKey.Hash(), alert))), submitRequest.Body)
		assert.Equal(t, baseUrl+"/issue/"+issueKey, submitRequest.URL)
		assert.Equal(t, "PUT", submitRequest.HTTPMethod)
	})
}

func mustMarshal(v interface{}) []byte {
	j, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return j
}
