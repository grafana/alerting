package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authtypes "github.com/grafana/authlib/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	"github.com/grafana/grafana-app-sdk/logging"
)

// testFilter builds a folder-only filter over the given accessible folder UIDs.
func testFilter(folders ...string) *ruleFilter {
	fset := make(ruleUIDSet, len(folders))
	for _, f := range folders {
		fset[f] = struct{}{}
	}
	return newRuleFilter(fset)
}

// fakeAuthInfo satisfies authtypes.AuthInfo for tests. Its methods are never
// invoked (the fake access client ignores the identity), so embedding the nil
// interface is sufficient to place a value on the context.
type fakeAuthInfo struct {
	authtypes.AuthInfo
}

// fakeAccessClient is a test double for authtypes.AccessClient that grants access
// to a fixed set of folder UIDs.
type fakeAccessClient struct {
	allowedFolders map[string]bool
	// gotChecks records the folder-scoped checks issued, for assertions.
	gotChecks []authtypes.BatchCheckItem
}

func (f *fakeAccessClient) Check(context.Context, authtypes.AuthInfo, authtypes.CheckRequest, string) (authtypes.CheckResponse, error) {
	return authtypes.CheckResponse{}, nil
}

func (f *fakeAccessClient) Compile(context.Context, authtypes.AuthInfo, authtypes.ListRequest) (authtypes.ItemChecker, authtypes.Zookie, error) {
	return func(_, folder string) bool { return f.allowedFolders[folder] }, &authtypes.NoopZookie{}, nil
}

func (f *fakeAccessClient) BatchCheck(_ context.Context, _ authtypes.AuthInfo, req authtypes.BatchCheckRequest) (authtypes.BatchCheckResponse, error) {
	results := make(map[string]authtypes.BatchCheckResult, len(req.Checks))
	for _, c := range req.Checks {
		f.gotChecks = append(f.gotChecks, c)
		results[c.CorrelationID] = authtypes.BatchCheckResult{Allowed: f.allowedFolders[c.Folder]}
	}
	return authtypes.BatchCheckResponse{Results: results}, nil
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
			filter: testFilter(),
			want:   "",
		},
		{
			name:   "single accessible folder",
			filter: testFilter("folderA"),
			want:   ` | folder_uids =~ "(^|.*,)(folderA)($|,.*)"`,
		},
		{
			name:   "multiple accessible folders are sorted",
			filter: testFilter("folderB", "folderA"),
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
			filter: testFilter(),
			want:   "",
		},
		{
			name:   "multiple accessible folders are sorted",
			filter: testFilter("folderB", "folderA"),
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
	filter := testFilter("folderA", "folderB")

	logql, err := buildQuery(Query{}, nil, filter)
	require.NoError(t, err)
	assert.Contains(t, logql, ` | folder_uids =~ "(^|.*,)(folderA|folderB)($|,.*)"`)
}

func TestBuildAlertQuery_RBACFolderFilterPushedDown(t *testing.T) {
	filter := testFilter("folderA", "folderB")

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

	// A non-nil but empty filter means the caller can access no folders.
	result, err := reader.Query(context.Background(), Query{}, testFilter())
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

	result, err := reader.QueryAlerts(context.Background(), AlertQuery{}, testFilter())
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

	result, err := reader.Query(context.Background(), Query{}, testFilter("folder-1"))
	require.NoError(t, err)
	assert.Len(t, result.Entries, 1)
	mockClient.AssertExpectations(t)
}

func TestExplodeRuleUIDCounts_SplitsCommaSeparated(t *testing.T) {
	// A notification referencing several rules is stored with a comma-separated
	// rule_uids label; grouping by rule must emit an individual count per rule,
	// each carrying the notification's count.
	counts := []Count{
		{RuleUID: stringPtr("ruleA,ruleB"), Count: 5},
	}

	got := explodeRuleUIDCounts(counts, 10)

	require.Len(t, got, 2)
	byUID := map[string]int64{}
	for _, c := range got {
		require.NotNil(t, c.RuleUID)
		byUID[*c.RuleUID] = c.Count
	}
	assert.Equal(t, int64(5), byUID["ruleA"])
	assert.Equal(t, int64(5), byUID["ruleB"])
}

func TestFolderAccessReader_AccessibleFolders(t *testing.T) {
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
					{"metadata": map[string]any{"name": "folder-a"}},
					{"metadata": map[string]any{"name": "folder-b"}},
				},
			}
		} else {
			resp = map[string]any{
				"metadata": map[string]any{"continue": ""},
				"items": []map[string]any{
					{"metadata": map[string]any{"name": "folder-c"}},
				},
			}
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	t.Cleanup(srv.Close)

	// The caller may read alert rules in folder-a and folder-c but not folder-b.
	access := &fakeAccessClient{allowedFolders: map[string]bool{"folder-a": true, "folder-c": true}}
	reader, err := newFolderAccessReader(rest.Config{Host: srv.URL}, access, &logging.NoOpLogger{})
	require.NoError(t, err)

	ctx := authtypes.WithAuthInfo(context.Background(), fakeAuthInfo{})
	folders, err := reader.AccessibleFolders(ctx, "org-1")
	require.NoError(t, err)

	assert.True(t, folders.Has("folder-a"))
	assert.False(t, folders.Has("folder-b"))
	assert.True(t, folders.Has("folder-c"))
	assert.Len(t, folders, 2)

	// Both pages should target the namespaced folders collection.
	require.Len(t, gotPaths, 2)
	for _, p := range gotPaths {
		assert.Equal(t, "/apis/folder.grafana.app/v1beta1/namespaces/org-1/folders", p)
	}

	// Every candidate folder is checked for alert.rules read access, folder-scoped.
	require.Len(t, access.gotChecks, 3)
	for _, c := range access.gotChecks {
		assert.Equal(t, rulesAPIGroup, c.Group)
		assert.Equal(t, alertRulesResource, c.Resource)
		assert.Equal(t, c.CorrelationID, c.Folder)
		assert.NotEmpty(t, c.Verb)
	}
}

func TestFolderAccessReader_RequiresAuthInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"metadata": map[string]any{"continue": ""},
			"items":    []map[string]any{{"metadata": map[string]any{"name": "folder-a"}}},
		}))
	}))
	t.Cleanup(srv.Close)

	access := &fakeAccessClient{allowedFolders: map[string]bool{"folder-a": true}}
	reader, err := newFolderAccessReader(rest.Config{Host: srv.URL}, access, &logging.NoOpLogger{})
	require.NoError(t, err)

	// No auth info on the context: the folder checks cannot be performed.
	_, err = reader.AccessibleFolders(context.Background(), "org-1")
	require.Error(t, err)
}

func TestNewFolderAccessReader_RequiresAccessClient(t *testing.T) {
	_, err := newFolderAccessReader(rest.Config{Host: "http://example"}, nil, &logging.NoOpLogger{})
	require.Error(t, err)
}

// fakeFolderAccessReader is a test double for folderAccessReader.
type fakeFolderAccessReader struct {
	folders ruleUIDSet
	err     error
}

func (f fakeFolderAccessReader) AccessibleFolders(_ context.Context, _ string) (ruleUIDSet, error) {
	return f.folders, f.err
}

func TestNotification_ResolveRuleFilter(t *testing.T) {
	t.Run("rbac disabled returns nil filter", func(t *testing.T) {
		n := &Notification{rbacEnabled: false}
		filter, err := n.resolveRuleFilter(context.Background(), "org-1")
		require.NoError(t, err)
		assert.Nil(t, filter)
	})

	t.Run("rbac enabled without reader fails closed", func(t *testing.T) {
		n := &Notification{rbacEnabled: true, folderAccess: nil}
		_, err := n.resolveRuleFilter(context.Background(), "org-1")
		require.Error(t, err)
	})

	t.Run("rbac enabled returns filter keyed by folder UIDs", func(t *testing.T) {
		n := &Notification{
			rbacEnabled:  true,
			folderAccess: fakeFolderAccessReader{folders: ruleUIDSet{"folderB": {}, "folderA": {}}},
		}
		filter, err := n.resolveRuleFilter(context.Background(), "org-1")
		require.NoError(t, err)
		require.NotNil(t, filter)
		// The push-down is keyed by the accessible folders, sorted.
		assert.Equal(t, []string{"folderA", "folderB"}, filter.folderKeys)
	})
}
