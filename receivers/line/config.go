package line

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

type Config struct {
	Token       string `json:"token,omitempty" yaml:"token,omitempty"`
	Title       string `json:"title,omitempty" yaml:"title,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

func BuildConfig(fc receivers.FactoryConfig) (*Config, error) {
	var settings Config
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	settings.Token = fc.DecryptFunc(context.Background(), fc.Config.SecureSettings, "token", settings.Token)
	if settings.Token == "" {
		return nil, errors.New("could not find token in settings")
	}
	if settings.Title == "" {
		settings.Title = templates.DefaultMessageTitleEmbed
	}
	if settings.Description == "" {
		settings.Description = templates.DefaultMessageEmbed
	}
	return &settings, nil
}
