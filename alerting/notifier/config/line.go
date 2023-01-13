package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/grafana/alerting/alerting/notifier/template"
)

type LineConfig struct {
	Token       string `json:"token,omitempty" yaml:"token,omitempty"`
	Title       string `json:"title,omitempty" yaml:"title,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

func BuildLineConfig(fc FactoryConfig) (*LineConfig, error) {
	var settings LineConfig
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	settings.Token = fc.DecryptFunc(context.Background(), fc.Config.SecureSettings, "token", settings.Token)
	if settings.Token == "" {
		return nil, errors.New("could not find token in settings")
	}
	if settings.Title == "" {
		settings.Title = template.DefaultMessageTitleEmbed
	}
	if settings.Description == "" {
		settings.Description = template.DefaultMessageEmbed
	}
	return &settings, nil
}
