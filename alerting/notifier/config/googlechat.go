package config

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/grafana/alerting/alerting/notifier/template"
)

type GoogleChatConfig struct {
	URL     string `json:"url,omitempty" yaml:"url,omitempty"`
	Title   string `json:"title,omitempty" yaml:"title,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

func BuildGoogleChatConfig(fc FactoryConfig) (*GoogleChatConfig, error) {
	var settings GoogleChatConfig
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.URL == "" {
		return nil, errors.New("could not find url property in settings")
	}
	if settings.Title == "" {
		settings.Title = template.DefaultMessageTitleEmbed
	}
	if settings.Message == "" {
		settings.Message = template.DefaultMessageEmbed
	}
	return &settings, nil
}
