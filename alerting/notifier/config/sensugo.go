package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/grafana/alerting/alerting/notifier/template"
)

type SensuGoSettings struct {
	URL       string `json:"url,omitempty" yaml:"url,omitempty"`
	Entity    string `json:"entity,omitempty" yaml:"entity,omitempty"`
	Check     string `json:"check,omitempty" yaml:"check,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Handler   string `json:"handler,omitempty" yaml:"handler,omitempty"`
	APIKey    string `json:"apikey,omitempty" yaml:"apikey,omitempty"`
	Message   string `json:"message,omitempty" yaml:"message,omitempty"`
}

func BuildSensuGoConfig(fc FactoryConfig) (SensuGoSettings, error) {
	settings := SensuGoSettings{}
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return settings, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	if settings.URL == "" {
		return settings, errors.New("could not find URL property in settings")
	}
	settings.APIKey = fc.DecryptFunc(context.Background(), fc.Config.SecureSettings, "apikey", settings.APIKey)
	if settings.APIKey == "" {
		return settings, errors.New("could not find the API key property in settings")
	}
	if settings.Message == "" {
		settings.Message = template.DefaultMessageEmbed
	}
	return settings, nil
}
