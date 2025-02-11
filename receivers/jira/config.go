package jira

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
)

type Config struct {
	URL string `yaml:"api_url,omitempty" json:"api_url,omitempty"`

	Project     string   `yaml:"project,omitempty" json:"project,omitempty"`
	Summary     string   `yaml:"summary,omitempty" json:"summary,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Labels      []string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Priority    string   `yaml:"priority,omitempty" json:"priority,omitempty"`
	IssueType   string   `yaml:"issue_type,omitempty" json:"issue_type,omitempty"`

	ReopenTransition  string         `yaml:"reopen_transition,omitempty" json:"reopen_transition,omitempty"`
	ResolveTransition string         `yaml:"resolve_transition,omitempty" json:"resolve_transition,omitempty"`
	WontFixResolution string         `yaml:"wont_fix_resolution,omitempty" json:"wont_fix_resolution,omitempty"`
	ReopenDuration    model.Duration `yaml:"reopen_duration,omitempty" json:"reopen_duration,omitempty"`

	Fields map[string]any `yaml:"fields,omitempty" json:"custom_fields,omitempty"`
}

func NewConfig(jsonData json.RawMessage) (Config, error) {
	settings := Config{}
	err := json.Unmarshal(jsonData, &settings)
	if err != nil {
		return settings, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.URL == "" {
		return settings, errors.New("could not find api_url property in settings")
	}
	if settings.Project == "" {
		return settings, fmt.Errorf("missing project in jira_config")
	}
	if settings.IssueType == "" {
		return settings, fmt.Errorf("missing issue_type in jira_config")
	}

	if settings.Summary == "" {
		settings.Summary = `{{ template "jira.default.summary" . }}`
	}
	if settings.Description == "" {
		settings.Description = `{{ template "jira.default.description" . }}`
	}
	if settings.Priority == "" {
		settings.Priority = `{{ template "jira.default.priority" . }}`
	}

	return settings, nil
}
