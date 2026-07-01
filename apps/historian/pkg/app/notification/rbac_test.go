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

func TestBuildNotificationRuleUIDsFilter(t *testing.T) {
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
			filter: newRuleFilter(ruleUIDSet{}),
			want:   "",
		},
		{
			name:   "single accessible rule",
			filter: newRuleFilter(ruleUIDSet{"ruleA": {}}),
			want:   ` | rule_uids =~ "(^|.*,)(ruleA)($|,.*)"`,
		},
		{
			name:   "multiple accessible rules are sorted",
			filter: newRuleFilter(ruleUIDSet{"ruleB": {}, "ruleA": {}}),
			want:   ` | rule_uids =~ "(^|.*,)(ruleA|ruleB)($|,.*)"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildNotificationRuleUIDsFilter(tt.filter))
		})
	}
}

func TestBuildAlertRuleUIDFilter(t *testing.T) {
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
			filter: newRuleFilter(ruleUIDSet{}),
			want:   "",
		},
		{
			name:   "multiple accessible rules are sorted",
			filter: newRuleFilter(ruleUIDSet{"ruleB": {}, "ruleA": {}}),
			want:   ` | rule_uid =~ "^(ruleA|ruleB)$"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildAlertRuleUIDFilter(tt.filter))
		})
	}
}

func TestBuildQuery_RBACFilterPushedDown(t *testing.T) {
	filter := newRuleFilter(ruleUIDSet{"ruleA": {}, "ruleB": {}})

	logql, err := buildQuery(Query{}, nil, filter)
	require.NoError(t, err)
	assert.Contains(t, logql, ` | rule_uids =~ "(^|.*,)(ruleA|ruleB)($|,.*)"`)
}

func TestBuildAlertQuery_RBACFilterPushedDown(t *testing.T) {
	filter := newRuleFilter(ruleUIDSet{"ruleA": {}, "ruleB": {}})

	logql, err := buildAlertQuery(AlertQuery{}, filter)
	require.NoError(t, err)
	assert.Contains(t, logql, ` | rule_uid =~ "^(ruleA|ruleB)$"`)
}

func TestLokiReader_Query_EmptyFilterReturnsEmptyWithoutQuerying(t *testing.T) {
	mockClient := &mockLokiClient{}
	reader := &LokiReader{
		client: mockClient,
		logger: &logging.NoOpLogger{},
	}

	// A non-nil but empty filter means the caller can access no rules.
	result, err := reader.Query(context.Background(), Query{}, newRuleFilter(ruleUIDSet{}))
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

	result, err := reader.QueryAlerts(context.Background(), AlertQuery{}, newRuleFilter(ruleUIDSet{}))
	require.NoError(t, err)
	assert.Empty(t, result.Alerts)
	mockClient.AssertNotCalled(t, "RangeQuery")
}

func TestLokiReader_Query_NonEmptyFilterAppliesMatcher(t *testing.T) {
	now := time.Now().UTC()
	mockClient := &mockLokiClient{}
	// Assert the pushed-down RBAC matcher is present in the LogQL sent to Loki.
	mockClient.On("RangeQuery", mock.Anything, mock.MatchedBy(func(logql string) bool {
		return strings.Contains(logql, `rule_uids =~ "(^|.*,)(rule-uid-1)($|,.*)"`)
	}), mock.Anything, mock.Anything, mock.Anything).
		Return(createMockLokiResponse(now.Add(-time.Hour)), nil)

	reader := &LokiReader{
		client: mockClient,
		logger: &logging.NoOpLogger{},
	}

	result, err := reader.Query(context.Background(), Query{}, newRuleFilter(ruleUIDSet{"rule-uid-1": {}}))
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
	filter := newRuleFilter(ruleUIDSet{"ruleA": {}})

	got := explodeRuleUIDCounts(counts, 10, filter)

	require.Len(t, got, 1)
	require.NotNil(t, got[0].RuleUID)
	assert.Equal(t, "ruleA", *got[0].RuleUID)
	assert.Equal(t, int64(5), got[0].Count)
}

func TestK8sRuleAccessReader_AccessibleRuleUIDs(t *testing.T) {
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
					{"metadata": map[string]any{"name": "rule-1"}},
					{"metadata": map[string]any{"name": "rule-2"}},
				},
			}
		} else {
			resp = map[string]any{
				"metadata": map[string]any{"continue": ""},
				"items": []map[string]any{
					{"metadata": map[string]any{"name": "rule-3"}},
				},
			}
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	t.Cleanup(srv.Close)

	reader, err := newK8sRuleAccessReader(rest.Config{Host: srv.URL}, &logging.NoOpLogger{})
	require.NoError(t, err)

	set, err := reader.AccessibleRuleUIDs(context.Background(), "org-1")
	require.NoError(t, err)

	assert.True(t, set.Has("rule-1"))
	assert.True(t, set.Has("rule-2"))
	assert.True(t, set.Has("rule-3"))
	assert.False(t, set.Has("rule-4"))
	assert.Len(t, set, 3)

	// Both pages should target the namespaced alertrules collection.
	require.Len(t, gotPaths, 2)
	for _, p := range gotPaths {
		assert.Equal(t, "/apis/rules.alerting.grafana.app/v0alpha1/namespaces/org-1/alertrules", p)
	}
}

// fakeRuleAccessReader is a test double for ruleAccessReader.
type fakeRuleAccessReader struct {
	set ruleUIDSet
	err error
}

func (f fakeRuleAccessReader) AccessibleRuleUIDs(_ context.Context, _ string) (ruleUIDSet, error) {
	return f.set, f.err
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

	t.Run("rbac enabled returns filter with accessible rules", func(t *testing.T) {
		n := &Notification{
			rbacEnabled: true,
			ruleAccess:  fakeRuleAccessReader{set: ruleUIDSet{"ruleA": {}, "ruleB": {}}},
		}
		filter, err := n.resolveRuleFilter(context.Background(), "org-1")
		require.NoError(t, err)
		require.NotNil(t, filter)
		assert.Equal(t, []string{"ruleA", "ruleB"}, filter.uids)
	})
}
