package config

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/grafana/alerting/alerting/notifier/template"
)

type DiscordConfig struct {
	Title              string `json:"title,omitempty" yaml:"title,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	AvatarURL          string `json:"avatar_url,omitempty" yaml:"avatar_url,omitempty"`
	WebhookURL         string `json:"url,omitempty" yaml:"url,omitempty"`
	UseDiscordUsername bool   `json:"use_discord_username,omitempty" yaml:"use_discord_username,omitempty"`
}

func BuildDiscordConfig(fc FactoryConfig) (*DiscordConfig, error) {
	var settings DiscordConfig
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	if settings.WebhookURL == "" {
		return nil, errors.New("could not find webhook url property in settings")
	}
	if settings.Title == "" {
		settings.Title = template.DefaultMessageTitleEmbed
	}
	if settings.Message == "" {
		settings.Message = template.DefaultMessageEmbed
	}
	return &settings, nil
}
