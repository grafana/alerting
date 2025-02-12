package jira

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestNotify(t *testing.T) {
	tmpl := templates.ForTests(t)

	exampleAlert := &types.Alert{
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

	serverV2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.Method, r.URL.Path)
		if r.Method == http.MethodGet && r.URL.Path == "/search" {
			w.Write([]byte(`{"issues": []}`))
		}
		if r.Method == http.MethodPost && r.URL.Path == "/issue" {
			w.Write([]byte(`{"key": "TEST-123"}`))
		}
	}))
	defer serverV2.Close()

	serverV3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/search" {
			w.Write([]byte(`{"issues": []}`))
		}
		if r.Method == http.MethodPost && r.URL.Path == "/issue" {
			w.Write([]byte(`{"key": "TEST-123"}`))
		}
	}))
	defer serverV3.Close()

	serverV2URL, err := url.Parse(serverV2.URL)
	require.NoError(t, err)
	serverV3URL, err := url.Parse(serverV3.URL)
	require.NoError(t, err)

	cases := []struct {
		name        string
		alerts      []*types.Alert
		apiVersion  string
		expHeaders  map[string]string
		expBody     string
		expURL      string
		statusCode  int
		expMsgError string
	}{
		{
			name:       "Single alert with v2 API creates issue",
			alerts:     []*types.Alert{exampleAlert},
			apiVersion: "2",
			expHeaders: map[string]string{
				"Content-Type":    "application/json",
				"Accept-Language": "en",
			},
			expURL:     "http://jira.example.com/issue",
			statusCode: http.StatusOK,
		},
		// {
		// 	name:       "Single alert with v3 API creates issue",
		// 	alerts:     []*types.Alert{exampleAlert},
		// 	apiVersion: "3",
		// 	expHeaders: map[string]string{
		// 		"Content-Type":    "application/json",
		// 		"Accept-Language": "en",
		// 	},
		// 	expURL:     "http://jira.example.com/3/issue",
		// 	statusCode: http.StatusOK,
		// },
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			webhookSender := receivers.MockNotificationService()
			webhookSender.StatusCode = c.statusCode
			webhookSender.ResponseBody = []byte(`{"key": "TEST-123"}`)

			baseURL := serverV2URL
			if c.apiVersion == "3" {
				baseURL = serverV3URL
			}

			cfg := Config{
				URL:         baseURL,
				Project:     "Test project",
				IssueType:   "Bug",
				Priority:    "High",
				Summary:     "{{ .CommonLabels.alertname }}",
				Description: "{{ .CommonAnnotations.description }}",
			}

			// Create notifier
			n := New(cfg, receivers.Metadata{}, tmpl, webhookSender, logging.FakeLogger{})

			// Test notification
			ctx := context.Background()
			ctx = notify.WithGroupKey(ctx, "test_group")

			ok, err := n.Notify(ctx, c.alerts...)

			if c.expMsgError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expMsgError)
				return
			}
			require.NoError(t, err)
			require.True(t, ok)
		})
	}
}

func TestPrepareIssueRequestBody(t *testing.T) {
	tmpl := templates.ForTests(t)

	tests := []struct {
		name       string
		conf       Config
		alerts     []*types.Alert
		expSummary string
		expError   string
	}{
		{
			name: "Default summary template",
			conf: Config{
				Project:   "PROJ",
				IssueType: "Bug",
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels: model.LabelSet{
							"alertname": "TestAlert",
						},
					},
				},
			},
			expSummary: "[FIRING:1] TestAlert",
		},
		{
			name: "Custom summary template",
			conf: Config{
				Project:   "PROJ",
				IssueType: "Bug",
				Summary:   "Custom: {{ .CommonLabels.alertname }}",
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels: model.LabelSet{
							"alertname": "TestAlert",
						},
					},
				},
			},
			expSummary: "Custom: TestAlert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := New(tt.conf, receivers.Metadata{}, tmpl, nil, logging.FakeLogger{})

			issue, err := n.prepareIssueRequestBody(context.Background(), logging.FakeLogger{}, "test_group", tt.alerts...)

			if tt.expError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expSummary, issue.Fields.Summary)
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
			name: "Default configuration without custom fields",
			conf: Config{
				Project: "TEST",
			},
			firing:      true,
			expectedJql: `statusCategory != Done and labels = "ALERT{group1}" and project="TEST" order by status ASC,resolutiondate DESC`,
		},
		{
			name: "Configuration with wont-fix resolution",
			conf: Config{
				Project:           "TEST",
				WontFixResolution: "won't fix",
			},
			firing:      true,
			expectedJql: `resolution != "won't fix" and statusCategory != Done and labels = "ALERT{group1}" and project="TEST" order by status ASC,resolutiondate DESC`,
		},
		{
			name: "Reopen transition is set",
			conf: Config{
				Project:          "TEST",
				ReopenTransition: "test",
			},
			firing:      true,
			expectedJql: `labels = "ALERT{group1}" and project="TEST" order by status ASC,resolutiondate DESC`,
		},
		{
			name: "Reopen duration specified",
			conf: Config{
				Project:        "TEST",
				ReopenDuration: model.Duration(30 * time.Minute),
			},
			firing:      false,
			expectedJql: `(resolutiondate is EMPTY OR resolutiondate >= -30m) and labels = "ALERT{group1}" and project="TEST" order by status ASC,resolutiondate DESC`,
		},
		{
			name: "Custom dedup key field",
			conf: Config{
				Project:           "TEST",
				DedupKeyFieldName: "12345",
			},
			firing:      true,
			expectedJql: `statusCategory != Done and (labels = "ALERT{group1}" or cf[12345] ~ "group1") and project="TEST" order by status ASC,resolutiondate DESC`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSearchJql(tt.conf, groupKey, tt.firing)
			require.Equal(t, tt.expectedJql, result.JQL)
		})
	}
}
