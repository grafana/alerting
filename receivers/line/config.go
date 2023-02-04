package line

import (
	"errors"
	"fmt"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

type Config struct {
	Token       receivers.Secret `json:"token,omitempty" yaml:"token,omitempty"`
	Title       string           `json:"title,omitempty" yaml:"title,omitempty"`
	Description string           `json:"description,omitempty" yaml:"description,omitempty"`
}

func ValidateConfig(fc receivers.FactoryConfig) (*Config, error) {
	var settings Config
	err := fc.Marshaller.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
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
