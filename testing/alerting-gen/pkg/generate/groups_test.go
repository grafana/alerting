package generate

import (
	"fmt"
	"math"
	"testing"

	models "github.com/grafana/grafana-openapi-client-go/models"
	"github.com/stretchr/testify/require"
)

func TestGenerateRules_CountsAndBasics(t *testing.T) {
	queryDS := "__expr__"
	writeDS := "prom"
	numAlerting := 3
	numRecording := 2
	seed := int64(4242)

	rules := GenerateRules(queryDS, writeDS, numAlerting, numRecording, seed)
	require.Len(t, rules, numAlerting+numRecording)

	var alerting, recording int
	for _, r := range rules {
		// All rules have at least one query using the provided queryDS
		require.NotEmpty(t, r.Data)
		for _, q := range r.Data {
			require.Equal(t, queryDS, q.DatasourceUID)
			require.NotEmpty(t, q.RefID)
			require.NotNil(t, q.Model)
		}

		require.NotNil(t, r.For)
		require.NotNil(t, r.Title)
		require.NotEmpty(t, r.UID)

		if r.Record != nil {
			recording++
			// Recording rules write to the provided writeDS and For == 0
			require.Equal(t, writeDS, r.Record.TargetDatasourceUID)
			require.NotNil(t, r.Record.Metric)
			require.Equal(t, int64(0), int64(*r.For))
		} else {
			alerting++
			require.NotNil(t, r.Condition)
		}
	}
	require.Equal(t, numAlerting, alerting)
	require.Equal(t, numRecording, recording)
}

func TestGroupRules_PartitionAndFolderCycling(t *testing.T) {
	// 4 rules -> 2 groups when rulesPerGroup=2
	rules := GenerateRules("__expr__", "prom", 3, 1, 100)
	require.Len(t, rules, 4)

	groups := GroupRules(rules, 2, 1, []string{"f1", "f2"}, 0, 100)
	require.Len(t, groups, 2)

	// Group 1 in f1, group 2 in f2 when groupsPerFolder=1
	require.Equal(t, "f1", groups[0].FolderUID)
	require.Len(t, groups[0].Rules, 2)
	require.Equal(t, "f2", groups[1].FolderUID)
	require.Len(t, groups[1].Rules, 2)

	// Each rule should be annotated with the group's folder and title
	for gi, g := range groups {
		require.NotEmpty(t, g.Rules)
		for _, r := range g.Rules {
			require.NotNil(t, r.FolderUID)
			require.Equal(t, g.FolderUID, *r.FolderUID)
			require.NotNil(t, r.RuleGroup)
			require.Equal(t, g.Title, *r.RuleGroup)
		}
		// Ensure we partitioned rule slice by 2
		if gi == 0 {
			require.Len(t, g.Rules, 2)
		}
	}
}

func TestGroupRules_DefaultsWhenZeroOrEmpty(t *testing.T) {
	// 5 rules; with defaults (rulesPerGroup<=0, groupsPerFolder<=0, empty folderUIDs)
	rules := GenerateRules("__expr__", "prom", 5, 0, 7)
	groups := GroupRules(rules, 0, 0, nil, 60, 7)
	require.Len(t, groups, 1)

	g := groups[0]
	require.Equal(t, int64(60), g.Interval)
	require.Equal(t, "default", g.FolderUID)
	require.Len(t, g.Rules, len(rules))

	// sanity: math ceiling is enforced when grouping is applied (not applicable here since one group)
	require.Equal(t, 1, int(math.Ceil(float64(len(rules))/float64(len(rules)))))
}

func TestGroupRules_MixedAlertingAndRecordingDistribution(t *testing.T) {
	// 3 alerting + 2 recording, rulesPerGroup=2 -> groups: [A,A], [A,R], [R]
	rules := GenerateRules("__expr__", "prom", 3, 2, 101)
	require.Len(t, rules, 5)

	groups := GroupRules(rules, 2, 1, []string{"f1", "f2"}, 0, 101)
	require.Len(t, groups, 3)

	// Check per-group recording counts match expected [0,1,1]
	expectedRecs := []int{0, 1, 1}
	for i, g := range groups {
		recs := 0
		for _, r := range g.Rules {
			if r.Record != nil {
				recs++
			}
		}
		require.Equal(t, expectedRecs[i], recs, "unexpected recording count in group %d", i)
	}

	// Ensure grouping preserves original rule order when flattened
	flat := make([]*models.ProvisionedAlertRule, 0, len(rules))
	for _, g := range groups {
		flat = append(flat, g.Rules...)
	}
	require.Equal(t, len(rules), len(flat))
	for i := range rules {
		require.Equal(t, rules[i].UID, flat[i].UID, "rule order changed at index %d", i)
	}
}

func TestGroupRules_NonEvenPartition(t *testing.T) {
	// 5 rules, rulesPerGroup=2 -> groups of sizes 2,2,1
	rules := GenerateRules("__expr__", "prom", 5, 0, 777)
	groups := GroupRules(rules, 2, 1, []string{"f"}, 0, 777)
	require.Len(t, groups, 3)
	require.Len(t, groups[0].Rules, 2)
	require.Len(t, groups[1].Rules, 2)
	require.Len(t, groups[2].Rules, 1)
}

func TestGroupRules_GroupsPerFolderGreaterThanOne(t *testing.T) {
	// 5 rules with rulesPerGroup=1 -> 5 groups
	rules := GenerateRules("__expr__", "prom", 5, 0, 42)
	groups := GroupRules(rules, 1, 2, []string{"f1", "f2"}, 0, 42)
	require.Len(t, groups, 5)
	// Expect folder assignment: f1, f1, f2, f2, f1 (two groups per folder before cycling)
	expected := []string{"f1", "f1", "f2", "f2", "f1"}
	for i, g := range groups {
		require.Equal(t, expected[i], g.FolderUID, "unexpected folder assignment at group %d", i)
	}
}

func TestGenerateRules_DeterminismWithSeed(t *testing.T) {
	q := "__expr__"
	w := "prom"
	nA, nR := 4, 3
	seed1 := int64(555)
	seed2 := int64(556)

	r1 := GenerateRules(q, w, nA, nR, seed1)
	r2 := GenerateRules(q, w, nA, nR, seed1)
	r3 := GenerateRules(q, w, nA, nR, seed2)

	sig1 := rulesSignature(r1)
	sig2 := rulesSignature(r2)
	sig3 := rulesSignature(r3)

	require.Equal(t, sig1, sig2, "same seed should produce identical rule signatures")
	require.NotEqual(t, sig1, sig3, "different seeds should produce different rule signatures")
}

// rulesSignature builds a deterministic string over selected stable fields to compare rule sets
func rulesSignature(rs []*models.ProvisionedAlertRule) string {
	out := ""
	for i, r := range rs {
		title := safeStr(r.Title)
		cond := safeStr(r.Condition)
		exec := safeStr(r.ExecErrState)
		noData := safeStr(r.NoDataState)
		org := safeInt64(r.OrgID)
		forDur := ""
		if r.For != nil {
			forDur = fmt.Sprintf("%d", int64(*r.For))
		}
		rec := "-"
		if r.Record != nil && r.Record.Metric != nil {
			rec = "rec:" + *r.Record.Metric + ":" + r.Record.TargetDatasourceUID
		}
		// include first query basics if present
		qsig := ""
		if len(r.Data) > 0 {
			q := r.Data[0]
			qsig = q.DatasourceUID + ":" + q.RefID
		}
		out += fmt.Sprintf("[%d]%s|%s|%s|%s|%s|%s|%s\n", i, title, r.UID, cond, forDur, exec, noData, org+"|"+rec+"|"+qsig)
	}
	return out
}

func safeStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func safeInt64(p *int64) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%d", *p)
}
