package dooray

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

type Config struct {
	Url         string `json:"url,omitempty" yaml:"url,omitempty"`
	Title       string `json:"title,omitempty" yaml:"title,omitempty"`
	IconURL     string `json:"icon_url,omitempty" yaml:"icon_url,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

func NewConfig(jsonData json.RawMessage, decryptFn receivers.DecryptFunc) (Config, error) {
	var settings Config
	err := json.Unmarshal(jsonData, &settings)
	if err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	settings.Url = decryptFn("url", settings.Url)
	if settings.Url == "" {
		return Config{}, errors.New("could not find url in settings")
	}
	if settings.Title == "" {
		settings.Title = templates.DefaultMessageTitleEmbed
	}
	if settings.Description == "" {
		settings.Description = templates.DefaultMessageEmbed
	}
	return settings, nil
}
