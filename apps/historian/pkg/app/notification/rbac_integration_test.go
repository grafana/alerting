package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/grafana/grafana-app-sdk/app"
	"github.com/grafana/grafana-app-sdk/logging"
	"github.com/grafana/grafana-app-sdk/resource"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/historian"
	"github.com/grafana/alerting/notify/historian/lokiclient"

	"github.com/grafana/alerting/apps/historian/pkg/apis/alertinghistorian/v0alpha1"
	"github.com/grafana/alerting/apps/historian/pkg/app/config"
)

// TestIntegration_NotificationRBAC drives the real notification query handlers
// against a live Loki, exercising the end-to-end RBAC push-down without needing
// Grafana or the alerting pipeline. It seeds a handful of synthetic
// notification-history entries under rule UIDs unique to this run, then queries
// as different "users" (via a fake rule-access reader) and asserts each user
// only sees history for the rules they can access.
//
// It is skipped unless HISTORIAN_LOKI_URL is set, e.g.:
//
//	docker run -d -p 3100:3100 grafana/loki:latest
//	HISTORIAN_LOKI_URL=http://localhost:3100 \
//	  go test ./apps/historian/pkg/app/notification/ -run TestIntegration_NotificationRBAC -v
//
// If your Loki uses multi-tenancy, also set HISTORIAN_LOKI_TENANT.
func TestIntegration_NotificationRBAC(t *testing.T) {
	lokiURL := os.Getenv("HISTORIAN_LOKI_URL")
	if lokiURL == "" {
		t.Skip("set HISTORIAN_LOKI_URL (e.g. http://localhost:3100) to run the notification RBAC integration test")
	}
	tenant := os.Getenv("HISTORIAN_LOKI_TENANT")

	base, err := url.Parse(lokiURL)
	require.NoError(t, err)

	// Rule/folder UIDs unique to this run, so assertions are independent of any
	// other data already present in Loki.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ruleA := "itrulea" + suffix
	ruleB := "itruleb" + suffix
	folderA := "itfoldera" + suffix
	folderB := "itfolderb" + suffix

	// Seed: two notifications per rule, plus one "mixed" notification that
	// references both rules (to prove the "access at least one" semantics).
	seeded := []notif{
		{uuid: "uuid-a1-" + suffix, ruleUIDs: []string{ruleA}, folderUIDs: []string{folderA}, receiver: "Team A", status: "firing", alertname: "AlertA"},
		{uuid: "uuid-a2-" + suffix, ruleUIDs: []string{ruleA}, folderUIDs: []string{folderA}, receiver: "Team A", status: "resolved", errStr: "boom", alertname: "AlertA"},
		{uuid: "uuid-b1-" + suffix, ruleUIDs: []string{ruleB}, folderUIDs: []string{folderB}, receiver: "Team B", status: "firing", alertname: "AlertB"},
		{uuid: "uuid-b2-" + suffix, ruleUIDs: []string{ruleB}, folderUIDs: []string{folderB}, receiver: "Team B", status: "resolved", errStr: "boom", alertname: "AlertB"},
		{uuid: "uuid-mix-" + suffix, ruleUIDs: []string{ruleA, ruleB}, folderUIDs: []string{folderA, folderB}, receiver: "Shared", status: "firing", alertname: "MixedGroup"},
	}
	pushSeed(t, lokiURL, tenant, seeded)

	// A single shared reader (one metric registration).
	reader := NewLokiReader(config.LokiConfig{
		LokiConfig: lokiclient.LokiConfig{
			ReadPathURL:    base,
			TenantID:       tenant,
			MaxQueryLength: 720 * time.Hour,
			MaxQuerySize:   65536,
		},
	}, prometheus.NewRegistry(), &logging.NoOpLogger{}, otel.GetTracerProvider().Tracer("test"))

	universe := ruleUIDSet{ruleA: {}, ruleB: {}}
	folderUniverse := ruleUIDSet{folderA: {}, folderB: {}}
	// newN builds an RBAC-enabled Notification. The push-down is folder-based, so
	// the accessible folder set alone drives visibility. Alert-rule RBAC is
	// strictly folder-scoped, so a notification that references an accessible
	// folder is fully accessible (no per-rule stripping).
	newN := func(folders ruleUIDSet) *Notification {
		return &Notification{
			loki:         reader,
			logger:       &logging.NoOpLogger{},
			rbacEnabled:  true,
			folderAccess: fakeFolderAccessReader{folders: folders},
		}
	}

	// Wait until all seeded entries are queryable (admin sees everything).
	admin := newN(folderUniverse)
	require.Eventually(t, func() bool {
		res := queryEntries(t, admin)
		return len(seededEntries(res.Entries, universe)) == len(seeded)
	}, 20*time.Second, 250*time.Millisecond, "seeded notification entries never became queryable")

	t.Run("user A sees folder A entries plus the mixed entry", func(t *testing.T) {
		res := queryEntries(t, newN(ruleUIDSet{folderA: {}}))
		entries := seededEntries(res.Entries, universe)

		uuids := entryUUIDs(entries)
		assert.ElementsMatch(t, []string{"uuid-a1-" + suffix, "uuid-a2-" + suffix, "uuid-mix-" + suffix}, uuids)
		for _, e := range entries {
			assert.Contains(t, e.RuleUIDs, ruleA, "every visible entry must reference rule A")
		}
		// RBAC is folder-scoped: the mixed notification references accessible
		// folder A, so it is fully visible including its co-referenced rule B.
		for _, e := range entries {
			if e.Uuid == "uuid-mix-"+suffix {
				assert.Contains(t, e.RuleUIDs, ruleB, "mixed entry is fully visible via folder A")
			}
		}
	})

	t.Run("user B sees folder B entries plus the mixed entry", func(t *testing.T) {
		res := queryEntries(t, newN(ruleUIDSet{folderB: {}}))
		entries := seededEntries(res.Entries, universe)

		uuids := entryUUIDs(entries)
		assert.ElementsMatch(t, []string{"uuid-b1-" + suffix, "uuid-b2-" + suffix, "uuid-mix-" + suffix}, uuids)
		for _, e := range entries {
			assert.Contains(t, e.RuleUIDs, ruleB, "every visible entry must reference rule B")
		}
	})

	t.Run("admin sees all seeded entries", func(t *testing.T) {
		res := queryEntries(t, newN(folderUniverse))
		assert.Len(t, seededEntries(res.Entries, universe), len(seeded))
	})

	t.Run("no accessible folders yields no entries", func(t *testing.T) {
		res := queryEntries(t, newN(ruleUIDSet{}))
		assert.Empty(t, seededEntries(res.Entries, universe))
	})

	t.Run("counts grouped by rule UID include co-referenced rules in accessible folders", func(t *testing.T) {
		res := queryCounts(t, newN(ruleUIDSet{folderA: {}}))

		var gotA, gotB int64
		for _, c := range res.Counts {
			if c.RuleUID == nil {
				continue
			}
			switch *c.RuleUID {
			case ruleA:
				gotA += c.Count
			case ruleB:
				gotB += c.Count
			}
		}
		// Two rule-A notifications + the mixed one all count towards rule A.
		assert.Equal(t, int64(3), gotA)
		// The mixed notification references accessible folder A, so it is fully
		// visible: its co-referenced rule B is counted too (folder-scoped RBAC).
		assert.Equal(t, int64(1), gotB)
	})

	t.Run("alerts query is filtered per folder UID", func(t *testing.T) {
		res := queryAlerts(t, newN(ruleUIDSet{folderA: {}}))

		var seen int
		for _, a := range res.Alerts {
			uid := a.Labels[models.RuleUIDLabel]
			if uid != ruleA && uid != ruleB {
				continue // not from this run
			}
			seen++
			assert.Equal(t, ruleA, uid, "user A must only see folder A alert lines")
		}
		// One alert from each rule-A notification, plus the rule-A alert from the
		// mixed notification (its rule-B alert line, in folder B, is filtered out).
		assert.Equal(t, 3, seen)
	})
}

// notif is a synthetic notification-history record to seed into Loki.
type notif struct {
	uuid       string
	ruleUIDs   []string
	folderUIDs []string
	receiver   string
	status     string
	errStr     string
	alertname  string
}

// pushSeed writes the given notifications to Loki in the exact stream/metadata
// shape the historian read path expects (see notify/historian/historian.go).
func pushSeed(t testing.TB, lokiURL, tenant string, notifs []notif) {
	t.Helper()

	now := time.Now().UTC()
	var eventValues, alertValues []lokiclient.Sample
	tick := 0
	nextTS := func() time.Time {
		tick++
		return now.Add(-time.Duration(tick) * time.Millisecond)
	}

	for _, n := range notifs {
		ts := nextTS()

		entry := historian.NotificationHistoryLokiEntry{
			SchemaVersion: historian.SchemaVersion,
			UUID:          n.uuid,
			RuleUIDs:      n.ruleUIDs,
			FolderUIDs:    n.folderUIDs,
			Receiver:      n.receiver,
			Integration:   "webhook",
			GroupKey:      "{}/{alertname=\"" + n.alertname + "\"}",
			Status:        n.status,
			GroupLabels:   map[string]string{"alertname": n.alertname},
			AlertCount:    len(n.ruleUIDs),
			Error:         n.errStr,
			Duration:      1_000_000,
			PipelineTime:  ts,
		}
		eventValues = append(eventValues, lokiclient.Sample{
			T: ts,
			V: mustJSON(t, entry),
			Metadata: map[string]string{
				"uuid":        n.uuid,
				"receiver":    n.receiver,
				"rule_uids":   joinComma(n.ruleUIDs),
				"folder_uids": joinComma(n.folderUIDs),
			},
		})

		// One alert line per referenced rule.
		for i, ruleUID := range n.ruleUIDs {
			folderUID := ruleUID
			if i < len(n.folderUIDs) {
				folderUID = n.folderUIDs[i]
			}
			ats := ts.Add(-time.Duration(i+1) * time.Microsecond)
			alert := historian.NotificationHistoryLokiEntryAlert{
				SchemaVersion: historian.SchemaVersion,
				UUID:          n.uuid,
				AlertIndex:    i,
				Status:        n.status,
				Labels:        map[string]string{models.RuleUIDLabel: ruleUID, "alertname": n.alertname},
				Annotations:   map[string]string{models.NamespaceUIDLabel: folderUID},
				StartsAt:      ats.Add(-30 * time.Minute),
				EndsAt:        ats.Add(-5 * time.Minute),
			}
			alertValues = append(alertValues, lokiclient.Sample{
				T: ats,
				V: mustJSON(t, alert),
				Metadata: map[string]string{
					"uuid":       n.uuid,
					"rule_uid":   ruleUID,
					"folder_uid": folderUID,
				},
			})
		}
	}

	body := struct {
		Streams []lokiclient.Stream `json:"streams"`
	}{
		Streams: []lokiclient.Stream{
			{Stream: map[string]string{historian.LabelFrom: historian.LabelFromValue}, Values: eventValues},
			{Stream: map[string]string{historian.LabelFrom: historian.LabelFromValueAlerts}, Values: alertValues},
		},
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, lokiURL+"/loki/api/v1/push", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if tenant != "" {
		req.Header.Set("X-Scope-OrgID", tenant)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		t.Fatalf("loki push failed: %d: %s", resp.StatusCode, msg)
	}
}

func queryEntries(t testing.TB, n *Notification) QueryResult {
	t.Helper()
	q := Query{}
	typ := v0alpha1.CreateNotificationqueryRequestBodyTypeEntries
	q.Type = &typ
	return doQuery(t, n, q)
}

func queryCounts(t testing.TB, n *Notification) QueryResult {
	t.Helper()
	q := Query{}
	typ := v0alpha1.CreateNotificationqueryRequestBodyTypeCounts
	q.Type = &typ
	q.GroupBy = &QueryGroupBy{RuleUID: true}
	return doQuery(t, n, q)
}

func doQuery(t testing.TB, n *Notification, body Query) QueryResult {
	t.Helper()
	rec := runHandler(t, n.QueryHandler, body)
	var res QueryResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	return res
}

func queryAlerts(t testing.TB, n *Notification) AlertQueryResult {
	t.Helper()
	rec := runHandler(t, n.QueryAlertsHandler, AlertQuery{})
	var res AlertQueryResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	return res
}

type handlerFn func(context.Context, app.CustomRouteResponseWriter, *app.CustomRouteRequest) error

func runHandler(t testing.TB, h handlerFn, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := &app.CustomRouteRequest{
		ResourceIdentifier: resource.FullIdentifier{Namespace: "default"},
		Body:               io.NopCloser(bytes.NewReader(raw)),
	}
	require.NoError(t, h(context.Background(), rec, req))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec
}

// seededEntries filters to entries referencing a rule from this test run.
func seededEntries(entries []Entry, universe ruleUIDSet) []Entry {
	var out []Entry
	for _, e := range entries {
		for _, uid := range e.RuleUIDs {
			if universe.Has(uid) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

func entryUUIDs(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Uuid)
	}
	return out
}

func mustJSON(t testing.TB, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}
