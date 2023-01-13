package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/grafana/alerting/alerting/notifier/template"
)

const (
	DefaultPagerDutySeverity = "critical"
	DefaultPagerDutyClass    = "default"
	DefaultPagerDutyGroup    = "default"
	DefaultPagerDutyClient   = "Grafana"
)

type PagerdutyConfig struct {
	Key           string            `json:"integrationKey,omitempty" yaml:"integrationKey,omitempty"`
	Severity      string            `json:"severity,omitempty" yaml:"severity,omitempty"`
	CustomDetails map[string]string `json:"-" yaml:"-"` // TODO support the settings in the config
	Class         string            `json:"class,omitempty" yaml:"class,omitempty"`
	Component     string            `json:"component,omitempty" yaml:"component,omitempty"`
	Group         string            `json:"group,omitempty" yaml:"group,omitempty"`
	Summary       string            `json:"summary,omitempty" yaml:"summary,omitempty"`
	Source        string            `json:"source,omitempty" yaml:"source,omitempty"`
	Client        string            `json:"client,omitempty" yaml:"client,omitempty"`
	ClientURL     string            `json:"client_url,omitempty" yaml:"client_url,omitempty"`
}

func BuildPagerdutyConfig(fc FactoryConfig) (*PagerdutyConfig, error) {
	settings := PagerdutyConfig{}
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	settings.Key = fc.DecryptFunc(context.Background(), fc.Config.SecureSettings, "integrationKey", settings.Key)
	if settings.Key == "" {
		return nil, errors.New("could not find integration key property in settings")
	}

	settings.CustomDetails = map[string]string{
		"firing":       `{{ template "__text_alert_list" .Alerts.Firing }}`,
		"resolved":     `{{ template "__text_alert_list" .Alerts.Resolved }}`,
		"num_firing":   `{{ .Alerts.Firing | len }}`,
		"num_resolved": `{{ .Alerts.Resolved | len }}`,
	}

	if settings.Severity == "" {
		settings.Severity = DefaultPagerDutySeverity
	}
	if settings.Class == "" {
		settings.Class = DefaultPagerDutyClass
	}
	if settings.Component == "" {
		settings.Component = "Grafana"
	}
	if settings.Group == "" {
		settings.Group = DefaultPagerDutyGroup
	}
	if settings.Summary == "" {
		settings.Summary = template.DefaultMessageTitleEmbed
	}
	if settings.Client == "" {
		settings.Client = DefaultPagerDutyClient
	}
	if settings.ClientURL == "" {
		settings.ClientURL = "{{ .ExternalURL }}"
	}
	if settings.Source == "" {
		source, err := os.Hostname()
		if err != nil {
			source = settings.Client
		}
		settings.Source = source
	}
	return &settings, nil
}
