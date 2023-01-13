package config

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/grafana/alerting/alerting/notifier/template"
)

type TeamsSettings struct {
	URL          string `json:"url,omitempty" yaml:"url,omitempty"`
	Message      string `json:"message,omitempty" yaml:"message,omitempty"`
	Title        string `json:"title,omitempty" yaml:"title,omitempty"`
	SectionTitle string `json:"sectiontitle,omitempty" yaml:"sectiontitle,omitempty"`
}

func BuildTeamsSettings(fc FactoryConfig) (TeamsSettings, error) {
	settings := TeamsSettings{}
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return settings, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	if settings.URL == "" {
		return settings, errors.New("could not find url property in settings")
	}
	if settings.Message == "" {
		settings.Message = `{{ template "teams.default.message" .}}`
	}
	if settings.Title == "" {
		settings.Title = template.DefaultMessageTitleEmbed
	}
	return settings, nil
}
