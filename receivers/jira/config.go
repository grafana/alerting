package jira

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/receivers"
)

var (
	DefaultSummary     = `{{ template "jira.default.summary" . }}`
	DefaultDescription = `{{ template "jira.default.description" . }}`
	DefaultPriority    = `{{ template "jira.default.priority" . }}`
)

type Config struct {
	URL *url.URL

	Project     string
	Summary     string
	Description string
	Labels      []string
	Priority    string
	IssueType   string

	ReopenTransition  string
	ResolveTransition string
	WontFixResolution string
	ReopenDuration    model.Duration

	Fields map[string]any

	User     string
	Password string
	Token    string
}

func NewConfig(jsonData json.RawMessage, decryptFn receivers.DecryptFunc) (Config, error) {
	type raw struct {
		URL               string         `yaml:"api_url,omitempty" json:"api_url,omitempty"`
		Project           string         `yaml:"project,omitempty" json:"project,omitempty"`
		Summary           string         `yaml:"summary,omitempty" json:"summary,omitempty"`
		Description       string         `yaml:"description,omitempty" json:"description,omitempty"`
		Labels            []string       `yaml:"labels,omitempty" json:"labels,omitempty"`
		Priority          string         `yaml:"priority,omitempty" json:"priority,omitempty"`
		IssueType         string         `yaml:"issue_type,omitempty" json:"issue_type,omitempty"`
		ReopenTransition  string         `yaml:"reopen_transition,omitempty" json:"reopen_transition,omitempty"`
		ResolveTransition string         `yaml:"resolve_transition,omitempty" json:"resolve_transition,omitempty"`
		WontFixResolution string         `yaml:"wont_fix_resolution,omitempty" json:"wont_fix_resolution,omitempty"`
		ReopenDuration    string         `yaml:"reopen_duration,omitempty" json:"reopen_duration,omitempty"`
		Fields            map[string]any `yaml:"fields,omitempty" json:"fields,omitempty"`
		User              string         `yaml:"username,omitempty" json:"username,omitempty"`
		Password          string         `yaml:"password,omitempty" json:"password,omitempty"`
		Token             string         `yaml:"api_token,omitempty" json:"api_token,omitempty"`
	}

	settings := raw{}
	err := json.Unmarshal(jsonData, &settings)
	if err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.URL == "" {
		return Config{}, errors.New("could not find api_url property in settings")
	}
	u, err := url.Parse(settings.URL)
	if err != nil {
		return Config{}, fmt.Errorf("field api_url is not a valid URL: %w", err)
	}

	var d model.Duration
	if settings.ReopenDuration != "" {
		d, err = model.ParseDuration(settings.ReopenDuration)
		if err != nil {
			return Config{}, fmt.Errorf("field reopen_duration is not a valid duration: %w", err)
		}
	}

	if settings.Project == "" {
		return Config{}, fmt.Errorf("missing project in jira_config")
	}
	if settings.IssueType == "" {
		return Config{}, fmt.Errorf("missing issue_type in jira_config")
	}

	if settings.Summary == "" {
		settings.Summary = DefaultSummary
	}
	if settings.Description == "" {
		settings.Description = DefaultDescription
	}
	if settings.Priority == "" {
		settings.Priority = DefaultPriority
	}

	// https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/#basic-auth-for-rest-apis
	settings.User = decryptFn("username", settings.User)
	settings.Password = decryptFn("password", settings.Password)
	settings.Token = decryptFn("api_token", settings.Token)
	if settings.Token == "" {
		if settings.User == "" || settings.Password == "" {
			return Config{}, errors.New("either username and password or api_token must be provided")
		}
	}

	return Config{
		URL:               u,
		Project:           settings.Project,
		Summary:           settings.Summary,
		Description:       settings.Description,
		Labels:            settings.Labels,
		Priority:          settings.Priority,
		IssueType:         settings.IssueType,
		ReopenTransition:  settings.ReopenTransition,
		ResolveTransition: settings.ResolveTransition,
		WontFixResolution: settings.WontFixResolution,
		ReopenDuration:    d,
		Fields:            settings.Fields,
		User:              settings.User,
		Password:          settings.Password,
		Token:             settings.Token,
	}, nil
}
