package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
			notifier := &Notifier{conf: conf}

			// Execute
			desc := notifier.prepareDescription(tt.description, log.NewNopLogger())
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
		{
			name: "Custom fields with template values",
			conf: Config{
				URL:         v2,
				Project:     "PROJ",
				IssueType:   "Bug",
				Summary:     "Test Summary",
				Description: "Test Description",
				Fields: map[string]any{
					"customfield_12345": "{{ .CommonLabels.severity }}",
					"customfield_67890": "Alert: {{ .CommonLabels.alertname }}",
					"customfield_11111": 42,
					"customfield_22222": true,
					"customfield_33333": "{{ .CommonAnnotations.ann1 }}",
				},
			},
			expPayload: `{
	          "fields": {
	            "customfield_11111": 42,
	            "customfield_12345": "critical",
	            "customfield_22222": true,
	            "customfield_33333": "annv1",
	            "customfield_67890": "Alert: alert1",
	            "description": "Test Description",
	            "issuetype": {
	              "name": "Bug"
	            },
	            "labels": [
	              "ALERT{test_group}"
	            ],
	            "project": {
	              "key": "PROJ"
	            },
	            "summary": "Test Summary"
	          }
	        }`,
		},
		{
			name: "Custom fields with invalid template fallback",
			conf: Config{
				URL:         v2,
				Project:     "PROJ",
				IssueType:   "Bug",
				Summary:     "Test Summary",
				Description: "Test Description",
				Fields: map[string]any{
					"customfield_12345": "{{ .CommonLabels.nonexistent }}",
					"customfield_67890": "{{ invalid template syntax",
					"customfield_11111": 123,
					"customfield_22222": "Static text",
				},
			},
			expPayload: `{
	          "fields": {
	            "customfield_11111": 123,
	            "customfield_12345": "",
	            "customfield_22222": "Static text",
	            "customfield_67890": "",
	            "description": "Test Description",
	            "issuetype": {
	              "name": "Bug"
	            },
	            "labels": [
	              "ALERT{test_group}"
	            ],
	            "project": {
	              "key": "PROJ"
	            },
	            "summary": "Test Summary"
	          }
	        }`,
		},
		{
			name: "No custom fields",
			conf: Config{
				URL:         v2,
				Project:     "PROJ",
				IssueType:   "Bug",
				Summary:     "Test Summary",
				Description: "Test Description",
				Fields:      nil,
			},
			expPayload: `{
	          "fields": {
	            "description": "Test Description",
	            "issuetype": {
	              "name": "Bug"
	            },
	            "labels": [
	              "ALERT{test_group}"
	            ],
	            "project": {
	              "key": "PROJ"
	            },
	            "summary": "Test Summary"
	          }
	        }`,
		},
		{
			name: "Custom fields with complex template expressions",
			conf: Config{
				URL:         v2,
				Project:     "PROJ",
				IssueType:   "Bug",
				Summary:     "Test Summary",
				Description: "Test Description",
				Fields: map[string]any{
					"customfield_status": "{{ if eq .Status \"firing\" }}ACTIVE{{ else }}RESOLVED{{ end }}",
					"customfield_count":  "Alert count: {{ len .Alerts }}",
					"customfield_nested": map[string]any{
						"id":   "{{ .CommonAnnotations.project }}",
						"name": "nested field",
					},
				},
			},
			expPayload: `{
	          "fields": {
	            "customfield_count": "Alert count: 1",
	            "customfield_nested": {
	              "id": "{{ .CommonAnnotations.project }}",
	              "name": "nested field"
	            },
	            "customfield_status": "ACTIVE",
	            "description": "Test Description",
	            "issuetype": {
	              "name": "Bug"
	            },
	            "labels": [
	              "ALERT{test_group}"
	            ],
	            "project": {
	              "key": "PROJ"
	            },
	            "summary": "Test Summary"
	          }
	        }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := New(tt.conf, receivers.Metadata{}, tmpl, nil, log.NewNopLogger())

			issue, err := n.prepareIssueRequestBody(context.Background(), log.NewNopLogger(), "test_group", alerts...)
			require.NoError(t, err)

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
	baseURL := "https://jira.example.com/2"
	baseURLv3 := "https://jira.example.com/3"

	t.Run("when firing", func(t *testing.T) {
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

		u, _ := url.Parse(baseURL)
		cfg := Config{
			URL:         u,
			Summary:     "sum",
			Description: "desc",
			Priority:    "high",
			User:        "test",
			Password:    "test",
		}

		t.Run("creates a new issue if no existing", func(t *testing.T) {
			mock := receivers.NewMockWebhookSender()
			mock.SendWebhookFunc = func(_ context.Context, cmd *receivers.SendWebhookSettings) error {
				switch cmd.URL {
				case baseURL + "/search":
					return cmd.Validation(mustMarshal(issueSearchResultV2{}), 200)
				case baseURL + "/issue":
					return cmd.Validation(nil, 201)
				default:
					t.Fatalf("unexpected url: %s", cmd.URL)
					return nil
				}
			}

			n := New(cfg, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
			retry, err := n.Notify(ctx, alert)
			require.NoError(t, err)
			require.False(t, retry)
			require.Len(t, mock.Calls, 2)

			searchRequest := mock.Calls[0].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, cfg.User, searchRequest.User)
			assert.Equal(t, cfg.Password, searchRequest.Password)
			assert.JSONEq(t, string(mustMarshal(getSearchJql(cfg, groupKey.Hash(), true))), searchRequest.Body)
			assert.Equal(t, baseURL+"/search", searchRequest.URL)
			assert.Equal(t, "POST", searchRequest.HTTPMethod)

			submitRequest := mock.Calls[1].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, cfg.User, submitRequest.User)
			assert.Equal(t, cfg.Password, submitRequest.Password)
			body, err := n.prepareIssueRequestBody(ctx, log.NewNopLogger(), groupKey.Hash(), alert)
			require.NoError(t, err)
			assert.JSONEq(t, string(mustMarshal(body)), submitRequest.Body)
			assert.Equal(t, baseURL+"/issue", submitRequest.URL)
			assert.Equal(t, "POST", submitRequest.HTTPMethod)
		})

		t.Run("updates existing issue if firing", func(t *testing.T) {
			mock := receivers.NewMockWebhookSender()
			issueKey := "TEST-1"
			mock.SendWebhookFunc = func(_ context.Context, cmd *receivers.SendWebhookSettings) error {
				switch cmd.URL {
				case baseURL + "/search":
					return cmd.Validation(mustMarshal(issueSearchResultV2{
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
				case baseURL + "/issue/" + issueKey:
					return cmd.Validation(nil, 201)
				default:
					t.Fatalf("unexpected url: %s", cmd.URL)
					return nil
				}
			}

			n := New(cfg, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
			retry, err := n.Notify(ctx, alert)
			require.NoError(t, err)
			require.False(t, retry)
			assert.Len(t, mock.Calls, 2)

			submitRequest := mock.Calls[1].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, cfg.User, submitRequest.User)
			assert.Equal(t, cfg.Password, submitRequest.Password)
			body, err := n.prepareIssueRequestBody(ctx, log.NewNopLogger(), groupKey.Hash(), alert)
			require.NoError(t, err)
			assert.JSONEq(t, string(mustMarshal(body)), submitRequest.Body)
			assert.Equal(t, baseURL+"/issue/"+issueKey, submitRequest.URL)
			assert.Equal(t, "PUT", submitRequest.HTTPMethod)
		})

		// v3 variants
		t.Run("v3: creates a new issue if no existing (uses /search/jql)", func(t *testing.T) {
			u3, _ := url.Parse(baseURLv3)
			cfg3 := cfg
			cfg3.URL = u3

			mock := receivers.NewMockWebhookSender()
			mock.SendWebhookFunc = func(_ context.Context, cmd *receivers.SendWebhookSettings) error {
				switch cmd.URL {
				case baseURLv3 + "/search/jql":
					// v3 response shape
					return cmd.Validation(mustMarshal(issueSearchResultV3{}), 200)
				case baseURLv3 + "/issue":
					return cmd.Validation(nil, 201)
				default:
					t.Fatalf("unexpected url: %s", cmd.URL)
					return nil
				}
			}

			n := New(cfg3, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
			retry, err := n.Notify(ctx, alert)
			require.NoError(t, err)
			require.False(t, retry)
			require.Len(t, mock.Calls, 2)

			searchRequest := mock.Calls[0].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, baseURLv3+"/search/jql", searchRequest.URL)
			assert.Equal(t, "POST", searchRequest.HTTPMethod)

			submitRequest := mock.Calls[1].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, baseURLv3+"/issue", submitRequest.URL)
			assert.Equal(t, "POST", submitRequest.HTTPMethod)
		})

		t.Run("v3: updates existing issue if firing (uses /search/jql)", func(t *testing.T) {
			u3, _ := url.Parse(baseURLv3)
			cfg3 := cfg
			cfg3.URL = u3

			mock := receivers.NewMockWebhookSender()
			issueKey := "TEST-1"
			mock.SendWebhookFunc = func(_ context.Context, cmd *receivers.SendWebhookSettings) error {
				switch cmd.URL {
				case baseURLv3 + "/search/jql":
					return cmd.Validation(mustMarshal(issueSearchResultV3{Issues: []issue{{
						Key:    issueKey,
						Fields: &issueFields{Status: &issueStatus{StatusCategory: keyValue{Key: "blah"}}},
					}}}), 200)
				case baseURLv3 + "/issue/" + issueKey:
					return cmd.Validation(nil, 201)
				default:
					t.Fatalf("unexpected url: %s", cmd.URL)
					return nil
				}
			}

			n := New(cfg3, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
			retry, err := n.Notify(ctx, alert)
			require.NoError(t, err)
			require.False(t, retry)
			assert.Len(t, mock.Calls, 2)

			submitRequest := mock.Calls[1].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, baseURLv3+"/issue/"+issueKey, submitRequest.URL)
			assert.Equal(t, "PUT", submitRequest.HTTPMethod)
		})

		t.Run("reopen the issue if it's status is done", func(t *testing.T) {
			cfg := cfg
			cfg.ReopenTransition = "quickly"

			// API presets
			transition := idNameValue{
				ID:   "1234",
				Name: cfg.ReopenTransition,
			}
			issueKey := "TEST-1"
			issueStatus := &issueStatus{
				StatusCategory: keyValue{
					Key: "done", // this should trigger reopen and transition
				},
			}

			mock := receivers.NewMockWebhookSender()
			mock.SendWebhookFunc = func(_ context.Context, cmd *receivers.SendWebhookSettings) error {
				switch cmd.URL {
				case baseURL + "/search":
					return cmd.Validation(mustMarshal(issueSearchResultV2{
						Issues: []issue{
							{
								Key: issueKey,
								Fields: &issueFields{
									Status: issueStatus,
								},
								Transition: nil,
							},
						},
					}), 200)
				case baseURL + "/issue/" + issueKey:
					return cmd.Validation(nil, 200)
				case baseURL + "/issue/" + issueKey + "/transitions":
					if cmd.HTTPMethod == "GET" {
						return cmd.Validation(mustMarshal(issueTransitions{
							Transitions: []idNameValue{
								transition,
							},
						}), 200)
					}
					return cmd.Validation(nil, 200)
				default:
					t.Fatalf("unexpected url: %s", cmd.URL)
					return nil
				}
			}

			n := New(cfg, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
			retry, err := n.Notify(ctx, alert)
			require.NoError(t, err)
			require.False(t, retry)
			assert.Len(t, mock.Calls, 4)

			submitRequest := mock.Calls[1].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, cfg.User, submitRequest.User)
			assert.Equal(t, cfg.Password, submitRequest.Password)
			body, err := n.prepareIssueRequestBody(ctx, log.NewNopLogger(), groupKey.Hash(), alert)
			require.NoError(t, err)
			assert.JSONEq(t, string(mustMarshal(body)), submitRequest.Body)
			assert.Equal(t, baseURL+"/issue/"+issueKey, submitRequest.URL)
			assert.Equal(t, "PUT", submitRequest.HTTPMethod)

			transitionsSearchRequest := mock.Calls[2].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, cfg.User, transitionsSearchRequest.User)
			assert.Equal(t, cfg.Password, transitionsSearchRequest.Password)
			assert.Equal(t, baseURL+fmt.Sprintf("/issue/%s/transitions", issueKey), transitionsSearchRequest.URL)
			assert.Equal(t, "GET", transitionsSearchRequest.HTTPMethod)

			transitionRequest := mock.Calls[3].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, cfg.User, transitionRequest.User)
			assert.Equal(t, cfg.Password, transitionRequest.Password)
			assert.Equal(t, baseURL+fmt.Sprintf("/issue/%s/transitions", issueKey), transitionRequest.URL)
			assert.Equal(t, "POST", transitionRequest.HTTPMethod)
			assert.JSONEq(t, string(mustMarshal(issue{
				Transition: &idNameValue{
					ID: transition.ID,
				},
			})), transitionRequest.Body)

			t.Run("do not reopen if ReopenTransition is empty", func(t *testing.T) {
				mock.Calls = nil
				cfg.ReopenTransition = ""

				n := New(cfg, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
				retry, err := n.Notify(ctx, alert)
				require.NoError(t, err)
				require.False(t, retry)
				assert.Len(t, mock.Calls, 2)

				submitRequest := mock.Calls[1].Args[2].(*receivers.SendWebhookSettings)
				assert.Equal(t, baseURL+"/issue/"+issueKey, submitRequest.URL)
				assert.Equal(t, "PUT", submitRequest.HTTPMethod)
			})

			t.Run("fail if transition is not found in supported (no-retry)", func(t *testing.T) {
				mock.Calls = nil
				cfg.ReopenTransition = "something-else"

				n := New(cfg, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
				retry, err := n.Notify(ctx, alert)
				require.Error(t, err)
				require.False(t, retry)

				assert.Len(t, mock.Calls, 3)

				submitRequest := mock.Calls[1].Args[2].(*receivers.SendWebhookSettings)
				assert.Equal(t, baseURL+"/issue/"+issueKey, submitRequest.URL)
				assert.Equal(t, "PUT", submitRequest.HTTPMethod)

				transitionsSearchRequest := mock.Calls[2].Args[2].(*receivers.SendWebhookSettings)
				assert.Equal(t, baseURL+fmt.Sprintf("/issue/%s/transitions", issueKey), transitionsSearchRequest.URL)
				assert.Equal(t, "GET", transitionsSearchRequest.HTTPMethod)
			})
		})
	})

	t.Run("when resolved", func(t *testing.T) {
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
				EndsAt: time.Now().Add(-1 * time.Second),
			},
		}

		u, _ := url.Parse(baseURL)
		cfg := Config{
			URL:         u,
			Summary:     "sum",
			Description: "desc",
			Priority:    "high",
			User:        "test",
			Password:    "test",
		}

		t.Run("does nothing if issue is not found", func(t *testing.T) {
			mock := receivers.NewMockWebhookSender()
			mock.SendWebhookFunc = func(_ context.Context, cmd *receivers.SendWebhookSettings) error {
				switch cmd.URL {
				case baseURL + "/search":
					return cmd.Validation(mustMarshal(issueSearchResultV2{}), 200)
				default:
					t.Fatalf("unexpected url: %s", cmd.URL)
					return nil
				}
			}

			n := New(cfg, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
			retry, err := n.Notify(ctx, alert)
			require.NoError(t, err)
			require.False(t, retry)
			require.Len(t, mock.Calls, 1)
		})

		t.Run("updates the issue even if it's resolved, no transition", func(t *testing.T) {
			// API presets
			issueKey := "TEST-1"
			issueStatus := &issueStatus{
				StatusCategory: keyValue{
					Key: "done",
				},
			}

			mock := receivers.NewMockWebhookSender()
			mock.SendWebhookFunc = func(_ context.Context, cmd *receivers.SendWebhookSettings) error {
				switch cmd.URL {
				case baseURL + "/search":
					return cmd.Validation(mustMarshal(issueSearchResultV2{
						Issues: []issue{
							{
								Key: issueKey,
								Fields: &issueFields{
									Status: issueStatus,
								},
							},
						},
					}), 200)
				case baseURL + "/issue/" + issueKey:
					return cmd.Validation(nil, 200)
				default:
					t.Fatalf("unexpected url: %s", cmd.URL)
					return nil
				}
			}

			n := New(cfg, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
			retry, err := n.Notify(ctx, alert)
			require.NoError(t, err)
			require.False(t, retry)
			require.Len(t, mock.Calls, 2)

			submitRequest := mock.Calls[1].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, cfg.User, submitRequest.User)
			assert.Equal(t, cfg.Password, submitRequest.Password)
			body, err := n.prepareIssueRequestBody(ctx, log.NewNopLogger(), groupKey.Hash(), alert)
			require.NoError(t, err)
			assert.JSONEq(t, string(mustMarshal(body)), submitRequest.Body)
			assert.Equal(t, baseURL+"/issue/"+issueKey, submitRequest.URL)
			assert.Equal(t, "PUT", submitRequest.HTTPMethod)
		})

		t.Run("updates the issue and transitions if it's not resolved", func(t *testing.T) {
			cfg := cfg
			cfg.ResolveTransition = "quickly"
			// API presets
			transition := idNameValue{
				ID:   "1234",
				Name: cfg.ResolveTransition,
			}
			issueKey := "TEST-1"
			issueStatus := &issueStatus{
				StatusCategory: keyValue{
					Key: "test", // should trigger resolve
				},
			}

			mock := receivers.NewMockWebhookSender()
			mock.SendWebhookFunc = func(_ context.Context, cmd *receivers.SendWebhookSettings) error {
				switch cmd.URL {
				case baseURL + "/search":
					return cmd.Validation(mustMarshal(issueSearchResultV2{
						Issues: []issue{
							{
								Key: issueKey,
								Fields: &issueFields{
									Status: issueStatus,
								},
							},
						},
					}), 200)
				case baseURL + "/issue/" + issueKey:
					return cmd.Validation(nil, 200)
				case baseURL + "/issue/" + issueKey + "/transitions":
					if cmd.HTTPMethod == "GET" {
						return cmd.Validation(mustMarshal(issueTransitions{
							Transitions: []idNameValue{
								transition,
							},
						}), 200)
					}
					return cmd.Validation(nil, 200)
				default:
					t.Fatalf("unexpected url: %s", cmd.URL)
					return nil
				}
			}

			n := New(cfg, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
			retry, err := n.Notify(ctx, alert)
			require.NoError(t, err)
			require.False(t, retry)
			require.Len(t, mock.Calls, 4)

			submitRequest := mock.Calls[1].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, cfg.User, submitRequest.User)
			assert.Equal(t, cfg.Password, submitRequest.Password)
			body, err := n.prepareIssueRequestBody(ctx, log.NewNopLogger(), groupKey.Hash(), alert)
			require.NoError(t, err)
			assert.JSONEq(t, string(mustMarshal(body)), submitRequest.Body)
			assert.Equal(t, baseURL+"/issue/"+issueKey, submitRequest.URL)
			assert.Equal(t, "PUT", submitRequest.HTTPMethod)

			transitionsSearchRequest := mock.Calls[2].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, cfg.User, transitionsSearchRequest.User)
			assert.Equal(t, cfg.Password, transitionsSearchRequest.Password)
			assert.Equal(t, baseURL+fmt.Sprintf("/issue/%s/transitions", issueKey), transitionsSearchRequest.URL)
			assert.Equal(t, "GET", transitionsSearchRequest.HTTPMethod)

			transitionRequest := mock.Calls[3].Args[2].(*receivers.SendWebhookSettings)
			assert.Equal(t, cfg.User, transitionRequest.User)
			assert.Equal(t, cfg.Password, transitionRequest.Password)
			assert.Equal(t, baseURL+fmt.Sprintf("/issue/%s/transitions", issueKey), transitionRequest.URL)
			assert.Equal(t, "POST", transitionRequest.HTTPMethod)
			assert.JSONEq(t, string(mustMarshal(issue{
				Transition: &idNameValue{
					ID: transition.ID,
				},
			})), transitionRequest.Body)

			t.Run("do not resolve if ResolveTransition is empty", func(t *testing.T) {
				mock.Calls = nil
				cfg.ResolveTransition = ""

				n := New(cfg, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
				retry, err := n.Notify(ctx, alert)
				require.NoError(t, err)
				require.False(t, retry)
				assert.Len(t, mock.Calls, 2)

				submitRequest := mock.Calls[1].Args[2].(*receivers.SendWebhookSettings)
				assert.Equal(t, baseURL+"/issue/"+issueKey, submitRequest.URL)
				assert.Equal(t, "PUT", submitRequest.HTTPMethod)
			})

			t.Run("fail if transition is not found in supported (no-retry)", func(t *testing.T) {
				mock.Calls = nil
				cfg.ResolveTransition = "something-else"

				n := New(cfg, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
				retry, err := n.Notify(ctx, alert)
				assert.Error(t, err)
				assert.False(t, retry)

				assert.Len(t, mock.Calls, 3)

				submitRequest := mock.Calls[1].Args[2].(*receivers.SendWebhookSettings)
				assert.Equal(t, baseURL+"/issue/"+issueKey, submitRequest.URL)
				assert.Equal(t, "PUT", submitRequest.HTTPMethod)

				transitionsSearchRequest := mock.Calls[2].Args[2].(*receivers.SendWebhookSettings)
				assert.Equal(t, baseURL+fmt.Sprintf("/issue/%s/transitions", issueKey), transitionsSearchRequest.URL)
				assert.Equal(t, "GET", transitionsSearchRequest.HTTPMethod)
			})
		})
		t.Run("v3: does nothing if issue is not found (uses /search/jql)", func(t *testing.T) {
			u3, _ := url.Parse(baseURLv3)
			cfg3 := cfg
			cfg3.URL = u3

			mock := receivers.NewMockWebhookSender()
			mock.SendWebhookFunc = func(_ context.Context, cmd *receivers.SendWebhookSettings) error {
				switch cmd.URL {
				case baseURLv3 + "/search/jql":
					return cmd.Validation(mustMarshal(issueSearchResultV3{}), 200)
				default:
					t.Fatalf("unexpected url: %s", cmd.URL)
					return nil
				}
			}

			n := New(cfg3, receivers.Metadata{}, tmpl, mock, log.NewNopLogger())
			retry, err := n.Notify(ctx, alert)
			require.NoError(t, err)
			require.False(t, retry)
			require.Len(t, mock.Calls, 1)
		})

	})
}

func mustMarshal(v interface{}) []byte {
	j, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return j
}
