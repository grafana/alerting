package config

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/grafana/alerting/alerting/notifier/template"
)

const (
	// DefaultVictoropsMessageType - Victorops uses "CRITICAL" string to indicate "Alerting" state
	DefaultVictoropsMessageType = "CRITICAL"
)

type VictorOpsSettings struct {
	URL         string `json:"url,omitempty" yaml:"url,omitempty"`
	MessageType string `json:"messageType,omitempty" yaml:"messageType,omitempty"`
	Title       string `json:"title,omitempty" yaml:"title,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

func BuildVictorOpsSettings(fc FactoryConfig) (VictorOpsSettings, error) {
	settings := VictorOpsSettings{}
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return settings, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	if settings.URL == "" {
		return settings, errors.New("could not find victorops url property in settings")
	}
	if settings.MessageType == "" {
		settings.MessageType = DefaultVictoropsMessageType
	}
	if settings.Title == "" {
		settings.Title = template.DefaultMessageTitleEmbed
	}
	if settings.Description == "" {
		settings.Description = template.DefaultMessageEmbed
	}
	return settings, nil
}
