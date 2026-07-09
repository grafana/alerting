package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	"github.com/grafana/grafana-app-sdk/logging"
)

// testFilter builds a filter over the given accessible rule and folder UIDs.
func testFilter(rules, folders []string) *ruleFilter {
	rset := make(ruleUIDSet, len(rules))
	for _, r := range rules {
		rset[r] = struct{}{}
	}
	fset := make(ruleUIDSet, len(folders))
	for _, f := range folders {
		fset[f] = struct{}{}
	}
	return newRuleFilter(accessScope{rules: rset, folders: fset})
}

func TestBuildNotificationFolderFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter *ruleFilter
		want   string
	}{
		{
			name:   "nil filter disables RBAC",
			filter: nil,
			want:   "",
		},
		{
			name:   "empty accessible set produces no matcher",
			filter: testFilter(nil, nil),
			want:   "",
		},
		{
			name:   "single accessible folder",
			filter: testFilter([]string{"ruleA"}, []string{"folderA"}),
			want:   ` | folder_uids =~ "(^|.*,)(folderA)($|,.*)"`,
		},
		{
			name:   "multiple accessible folders are sorted",
			filter: testFilter([]string{"ruleA"}, []string{"folderB", "folderA"}),
			want:   ` | folder_uids =~ "(^|.*,)(folderA|folderB)($|,.*)"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildNotificationFolderFilter(tt.filter))
		})
	}
}

func TestBuildAlertFolderFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter *ruleFilter
		want   string
	}{
		{
			name:   "nil filter disables RBAC",
			filter: nil,
			want:   "",
		},
		{
			name:   "empty accessible set produces no matcher",
			filter: testFilter(nil, nil),
			want:   "",
		},
		{
			name:   "multiple accessible folders are sorted",
			filter: testFilter([]string{"ruleA"}, []string{"folderB", "folderA"}),
			want:   ` | folder_uid =~ "^(folderA|folderB)$"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildAlertFolderFilter(tt.filter))
		})
	}
}

func TestBuildQuery_RBACFolderFilterPushedDown(t *testing.T) {
	filter := testFilter([]string{"ruleA"}, []string{"folderA", "folderB"})

	logql, err := buildQuery(Query{}, nil, filter)
	require.NoError(t, err)
	assert.Contains(t, logql, ` | folder_uids =~ "(^|.*,)(folderA|folderB)($|,.*)"`)
}

func TestBuildAlertQuery_RBACFolderFilterPushedDown(t *testing.T) {
	filter := testFilter([]string{"ruleA"}, []string{"folderA", "folderB"})

	logql, err := buildAlertQuery(AlertQuery{}, filter)
	require.NoError(t, err)
	assert.Contains(t, logql, ` | folder_uid =~ "^(folderA|folderB)$"`)
}

func TestLokiReader_Query_EmptyFilterReturnsEmptyWithoutQuerying(t *testing.T) {
	mockClient := &mockLokiClient{}
	reader := &LokiReader{
		client: mockClient,
		logger: &logging.NoOpLogger{},
	}

	// A non-nil but empty filter means the caller can access no rules.
	result, err := reader.Query(context.Background(), Query{}, testFilter(nil, nil))
	require.NoError(t, err)
	assert.Empty(t, result.Entries)
	assert.Empty(t, result.Counts)
	mockClient.AssertNotCalled(t, "RangeQuery")
	mockClient.AssertNotCalled(t, "MetricsQuery")
	mockClient.AssertNotCalled(t, "MetricsRangeQuery")
}

func TestLokiReader_QueryAlerts_EmptyFilterReturnsEmptyWithoutQuerying(t *testing.T) {
	mockClient := &mockLokiClient{}
	reader := &LokiReader{
		client: mockClient,
		logger: &logging.NoOpLogger{},
	}

	result, err := reader.QueryAlerts(context.Background(), AlertQuery{}, testFilter(nil, nil))
	require.NoError(t, err)
	assert.Empty(t, result.Alerts)
	mockClient.AssertNotCalled(t, "RangeQuery")
}

func TestLokiReader_Query_NonEmptyFilterAppliesMatcher(t *testing.T) {
	now := time.Now().UTC()
	mockClient := &mockLokiClient{}
	// Assert the pushed-down RBAC matcher is present in the LogQL sent to Loki.
	mockClient.On("RangeQuery", mock.Anything, mock.MatchedBy(func(logql string) bool {
		return strings.Contains(logql, `folder_uids =~ "(^|.*,)(folder-1)($|,.*)"`)
	}), mock.Anything, mock.Anything, mock.Anything).
		Return(createMockLokiResponse(now.Add(-time.Hour)), nil)

	reader := &LokiReader{
		client: mockClient,
		logger: &logging.NoOpLogger{},
	}

	result, err := reader.Query(context.Background(), Query{}, testFilter([]string{"rule-uid-1"}, []string{"folder-1"}))
	require.NoError(t, err)
	assert.Len(t, result.Entries, 1)
	mockClient.AssertExpectations(t)
}

func TestExplodeRuleUIDCounts_DropsInaccessibleRules(t *testing.T) {
	// A notification referencing both an accessible (ruleA) and inaccessible (ruleB)
	// rule must only produce a per-rule count for the accessible one.
	counts := []Count{
		{RuleUID: stringPtr("ruleA,ruleB"), Count: 5},
	}
	filter := testFilter([]string{"ruleA"}, []string{"folderA"})

	got := explodeRuleUIDCounts(counts, 10, filter)

	require.Len(t, got, 1)
	require.NotNil(t, got[0].RuleUID)
	assert.Equal(t, "ruleA", *got[0].RuleUID)
	assert.Equal(t, int64(5), got[0].Count)
}

func TestK8sRuleAccessReader_AccessibleScope(t *testing.T) {
	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		cont := r.URL.Query().Get("continue")
		var resp map[string]any
		if cont == "" {
			// First page returns a continue token.
			resp = map[string]any{
				"metadata": map[string]any{"continue": "next-page"},
				"items": []map[string]any{
					{"metadata": map[string]any{"name": "rule-1", "labels": map[string]any{folderLabelKey: "folder-a"}}},
					{"metadata": map[string]any{"name": "rule-2", "labels": map[string]any{folderLabelKey: "folder-a"}}},
				},
			}
		} else {
			resp = map[string]any{
				"metadata": map[string]any{"continue": ""},
				"items": []map[string]any{
					{"metadata": map[string]any{"name": "rule-3", "labels": map[string]any{folderLabelKey: "folder-b"}}},
				},
			}
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	t.Cleanup(srv.Close)

	reader, err := newK8sRuleAccessReader(rest.Config{Host: srv.URL}, &logging.NoOpLogger{})
	require.NoError(t, err)

	scope, err := reader.AccessibleScope(context.Background(), "org-1")
	require.NoError(t, err)

	assert.True(t, scope.rules.Has("rule-1"))
	assert.True(t, scope.rules.Has("rule-2"))
	assert.True(t, scope.rules.Has("rule-3"))
	assert.False(t, scope.rules.Has("rule-4"))
	assert.Len(t, scope.rules, 3)

	// Folders are collected from the grafana.app/folder label and de-duplicated.
	assert.True(t, scope.folders.Has("folder-a"))
	assert.True(t, scope.folders.Has("folder-b"))
	assert.Len(t, scope.folders, 2)

	// Both pages should target the namespaced alertrules collection.
	require.Len(t, gotPaths, 2)
	for _, p := range gotPaths {
		assert.Equal(t, "/apis/rules.alerting.grafana.app/v0alpha1/namespaces/org-1/alertrules", p)
	}
}

// fakeRuleAccessReader is a test double for ruleAccessReader.
type fakeRuleAccessReader struct {
	scope accessScope
	err   error
}

func (f fakeRuleAccessReader) AccessibleScope(_ context.Context, _ string) (accessScope, error) {
	return f.scope, f.err
}

func TestNotification_ResolveRuleFilter(t *testing.T) {
	t.Run("rbac disabled returns nil filter", func(t *testing.T) {
		n := &Notification{rbacEnabled: false}
		filter, err := n.resolveRuleFilter(context.Background(), "org-1")
		require.NoError(t, err)
		assert.Nil(t, filter)
	})

	t.Run("rbac enabled without reader fails closed", func(t *testing.T) {
		n := &Notification{rbacEnabled: true, ruleAccess: nil}
		_, err := n.resolveRuleFilter(context.Background(), "org-1")
		require.Error(t, err)
	})

	t.Run("rbac enabled returns filter keyed by folder UIDs", func(t *testing.T) {
		n := &Notification{
			rbacEnabled: true,
			ruleAccess: fakeRuleAccessReader{scope: accessScope{
				rules:   ruleUIDSet{"ruleA": {}, "ruleB": {}},
				folders: ruleUIDSet{"folderB": {}, "folderA": {}},
			}},
		}
		filter, err := n.resolveRuleFilter(context.Background(), "org-1")
		require.NoError(t, err)
		require.NotNil(t, filter)
		// The push-down is keyed by accessible folders...
		assert.Equal(t, []string{"folderA", "folderB"}, filter.folderKeys)
		// ...while the rule set is retained for per-rule count stripping.
		assert.True(t, filter.rules.Has("ruleA"))
		assert.True(t, filter.rules.Has("ruleB"))
	})
}
