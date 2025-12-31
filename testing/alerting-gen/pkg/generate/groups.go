package generate

import (
	"fmt"

	models "github.com/grafana/grafana-openapi-client-go/models"
	"pgregory.net/rapid"
)

// GenerateRules creates alerting and recording rules using the lower-level generators.
func GenerateRules(queryDS, writeDS string, numAlerting, numRecording int, seed int64) []*models.ProvisionedAlertRule {
	alertGen := NewAlertingRuleGenerator(queryDS)
	recGen := NewRecordingRuleGenerator(queryDS, writeDS)

	rules := make([]*models.ProvisionedAlertRule, 0, numAlerting+numRecording)
	if numAlerting > 0 {
		alertingRules := rapid.SliceOfN(alertGen, numAlerting, numAlerting).Example(int(seed))
		rules = append(rules, alertingRules...)
	}
	if numRecording > 0 {
		recordingRules := rapid.SliceOfN(recGen, numRecording, numRecording).Example(int(seed))
		rules = append(rules, recordingRules...)
	}
	return rules
}

// GroupRules partitions rules into AlertRuleGroups and attaches FolderUID and RuleGroup on each rule.
func GroupRules(rules []*models.ProvisionedAlertRule, rulesPerGroup, groupsPerFolder int, folderUIDs []string, interval, seed int64) []*models.AlertRuleGroup {
	if rulesPerGroup <= 0 {
		rulesPerGroup = len(rules)
	}
	if groupsPerFolder <= 0 {
		groupsPerFolder = 1
	}
	if len(folderUIDs) == 0 {
		folderUIDs = []string{"default"}
	}

	groups := make([]*models.AlertRuleGroup, 0)
	groupIdx := 0
	for i := 0; i < len(rules); i += rulesPerGroup {
		end := min(i+rulesPerGroup, len(rules))
		name := fmt.Sprintf("group-%d", groupIdx+1)
		folderUID := folderUIDs[(groupIdx/groupsPerFolder)%len(folderUIDs)]

		// Use fixed interval if provided, otherwise generate random (1-20 minutes, divisible by 10).
		groupEvalInterval := interval
		if groupEvalInterval == 0 {
			groupEvalInterval = rapid.Int64Range(6, 120).Example(int(seed)+groupIdx) * 10
		}

		slice := rules[i:end]
		for _, r := range slice {
			r.RuleGroup = strPtr(name)
			r.FolderUID = strPtr(folderUID)
		}

		g := &models.AlertRuleGroup{
			FolderUID: folderUID,
			Interval:  groupEvalInterval,
			Rules:     slice,
			Title:     name,
		}
		groups = append(groups, g)
		groupIdx++
	}
	return groups
}

// GenerateGroups produces provisioning groups by combining rule generation and grouping.
func GenerateGroups(cfg Config) ([]*models.AlertRuleGroup, error) {
	rules := GenerateRules(cfg.QueryDS, cfg.WriteDS, cfg.AlertRuleCount, cfg.RecordingRuleCount, cfg.Seed)
	groups := GroupRules(rules, cfg.RulesPerGroup, cfg.GroupsPerFolder, cfg.FolderUIDs, cfg.EvalInterval, cfg.Seed)
	return groups, nil
}
