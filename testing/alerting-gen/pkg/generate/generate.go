package generate

import (
	"time"

	"github.com/go-openapi/strfmt"
	models "github.com/grafana/grafana-openapi-client-go/models"
	"pgregory.net/rapid"
)

type Config struct {
	NumAlerting     int
	NumRecording    int
	QueryDS         string
	WriteDS         string
	RulesPerGroup   int
	GroupsPerFolder int
	Seed            int64
	FolderUIDs      []string
}

// GenerateGroups moved to groups.go

// Helpers moved to helpers.go

// NewAlertingRuleGenerator returns a rapid generator for alerting rules
func NewAlertingRuleGenerator(queryDs string) *rapid.Generator[*models.ProvisionedAlertRule] {
	return rapid.Custom(func(t *rapid.T) *models.ProvisionedAlertRule {
		// local refID scoped to this rule
		refID := "A"
		data := []*models.AlertQuery{buildQuery(queryDs, refID)}
		title := genTitle().Draw(t, "title")
		forDur := mustParseDuration(genDurationStr().Draw(t, "for"))
		// KeepFiringFor must be a multiple of For (0 inclusive)
		var keepDur strfmt.Duration
		if time.Duration(forDur) == 0 {
			keepDur = strfmt.Duration(0)
		} else {
			mult := rapid.IntRange(0, 5).Draw(t, "keep_mult")
			keepDur = strfmt.Duration(time.Duration(forDur) * time.Duration(mult))
		}
		uid := randomUID().Draw(t, "uid")
		labels := genLabels().Draw(t, "labels")
		summary := genSummary().Draw(t, "summary")
		extraAnns := genAdditionalAnnotations().Draw(t, "annotations")
		anns := map[string]string{"summary": summary}
		for k, v := range extraAnns {
			if k == "summary" {
				continue
			}
			anns[k] = v
		}
		execErr := genExecErrState().Draw(t, "exec_err_state")
		noData := genNoDataState().Draw(t, "no_data_state")
		paused := rapid.Bool().Draw(t, "is_paused")
		missingToResolve := rapid.Int64Range(0, 5).Draw(t, "missing_to_resolve")
		// TODO: make orgID configurable; assume 1 for now
		orgID := int64(1)

		return &models.ProvisionedAlertRule{
			Title:                       strPtr(title),
			UID:                         uid,
			RuleGroup:                   nil, // filled when grouped
			FolderUID:                   nil, // filled when grouped
			Condition:                   strPtr(refID),
			Data:                        data,
			For:                         &forDur,
			KeepFiringFor:               keepDur,
			IsPaused:                    paused,
			ExecErrState:                strPtr(execErr),
			NoDataState:                 strPtr(noData),
			Labels:                      labels,
			Annotations:                 anns,
			MissingSeriesEvalsToResolve: missingToResolve,
			OrgID:                       &orgID,
		}
	})
}

// NewRecordingRuleGenerator returns a rapid generator for recording rules
func NewRecordingRuleGenerator(queryDS, writeDS string) *rapid.Generator[*models.ProvisionedAlertRule] {
	return rapid.Custom(func(t *rapid.T) *models.ProvisionedAlertRule {
		// local refID scoped to this rule
		refID := "A"
		data := []*models.AlertQuery{buildQuery(queryDS, refID)}
		title := genTitle().Draw(t, "title")
		uid := randomUID().Draw(t, "uid")
		metric := genMetricName().Draw(t, "metric")
		summary := genSummary().Draw(t, "summary")
		extraAnns := genAdditionalAnnotations().Draw(t, "annotations")
		anns := map[string]string{"summary": summary}
		for k, v := range extraAnns {
			if k == "summary" {
				continue
			}
			anns[k] = v
		}
		paused := rapid.Bool().Draw(t, "is_paused")
		// TODO: make orgID configurable; assume 1 for now
		orgID := int64(1)

		return &models.ProvisionedAlertRule{
			Title:       strPtr(title),
			UID:         uid,
			RuleGroup:   nil,
			FolderUID:   nil,
			Data:        data,
			IsPaused:    paused,
			Labels:      map[string]string{"rule_kind": "recording"},
			Annotations: anns,
			Record: &models.Record{
				From:                strPtr(refID),
				Metric:              strPtr(metric),
				TargetDatasourceUID: writeDS,
			},
			OrgID: &orgID,
		}
	})
}

// helper functions are defined in helpers.go
