package generate

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestAlertingRuleGenerator_Properties(t *testing.T) {
	queryDs := "__expr__"
	gen := NewAlertingRuleGenerator(queryDs)

	rapid.Check(t, func(t *rapid.T) {
		// sample deterministically with the PBT engine seed
		rule := gen.Draw(t, "rule")

		require.Nil(t, rule.RuleGroup, "RuleGroup must be nil in generator output")
		require.Nil(t, rule.FolderUID, "FolderUID must be nil in generator output")

		require.NotNil(t, rule.Condition, "Condition must be set")
		require.NotEmpty(t, *rule.Condition, "Condition must be non-empty")

		require.NotEmpty(t, rule.Data, "Data must be non-empty")
		for _, query := range rule.Data {
			require.Equal(t, queryDs, query.DatasourceUID)
			require.NotEmpty(t, query.RefID, "AlertQuery RefID must be set")
			require.NotNil(t, query.Model, "AlertQuery Model must be set")
		}

		require.NotNil(t, rule.For, "For must be set")
		// KeepFiringFor must be a multiple of For (0 inclusive)
		if *rule.For == 0 {
			require.Equal(t, int64(0), int64(rule.KeepFiringFor), "keep_firing_for must be 0 when for is 0")
		} else {
			require.Equal(t, int64(0), int64(rule.KeepFiringFor%(*rule.For)), "keep_firing_for must be a multiple of for")
		}

		require.NotNil(t, rule.ExecErrState, "ExecErrState must be set")
		require.Contains(t, []string{"OK", "Alerting", "Error"}, *rule.ExecErrState, "ExecErrState must be one of allowed values")

		require.NotNil(t, rule.NoDataState, "NoDataState must be set")
		require.Contains(t, []string{"OK", "Alerting", "NoData"}, *rule.NoDataState, "NoDataState must be one of allowed values")

		require.NotNil(t, rule.OrgID)
		require.Equal(t, int64(1), *rule.OrgID, "OrgID must be 1 for now")

		require.NotNil(t, rule.Title)
		require.NotEmpty(t, *rule.Title)
		require.NotEmpty(t, rule.UID)

		require.NotNil(t, rule.Annotations)
		require.Contains(t, rule.Annotations, "summary")
	})
}

func TestRecordingRuleGenerator_Properties(t *testing.T) {
	queryDS := "__expr__"
	writeDS := "prom"
	gen := NewRecordingRuleGenerator(queryDS, writeDS)

	rapid.Check(t, func(t *rapid.T) {
		rule := gen.Draw(t, "rule")

		require.NotNil(t, rule.Record)
		require.NotNil(t, rule.Record.Metric)
		require.NotEmpty(t, *rule.Record.Metric, "Recording rule must set metric")
		require.Equal(t, writeDS, rule.Record.TargetDatasourceUID)

		require.NotNil(t, rule.For, "Recording rules still require For pointer")
		require.Equal(t, int64(0), int64(*rule.For), "Recording rules For expected 0")

		require.NotNil(t, rule.OrgID)
		require.Equal(t, int64(1), *rule.OrgID, "OrgID must be 1 for now")

		require.NotNil(t, rule.Annotations)
		require.Contains(t, rule.Annotations, "summary")
	})
}

func TestGenerateGroups_SetsFolderAndGroup(t *testing.T) {
	cfg := Config{
		AlertRuleCount:     3,
		RecordingRuleCount: 2,
		QueryDS:            "__expr__",
		WriteDS:            "prom",
		RulesPerGroup:      2,
		GroupsPerFolder:    2,
		Seed:               1234,
		FolderUIDs:         []string{"f1", "f2"},
	}
	groups, err := GenerateGroups(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, groups)
	for _, g := range groups {
		require.NotEmpty(t, g.FolderUID, "group folder UID must be set")
		require.NotEmpty(t, g.Title, "group title must be set")
		require.NotEmpty(t, g.Rules, "group must contain rules")
		for _, r := range g.Rules {
			require.NotNil(t, r.FolderUID)
			require.Equal(t, g.FolderUID, *r.FolderUID, "rule FolderUID not set to group folder")
			require.NotNil(t, r.RuleGroup)
			require.Equal(t, g.Title, *r.RuleGroup, "rule RuleGroup not set to group title")
		}
	}
}
