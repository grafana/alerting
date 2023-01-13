package victorops

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

const (
	// DefaultVictoropsMessageType - Victorops uses "CRITICAL" string to indicate "Alerting" state
	DefaultVictoropsMessageType = "CRITICAL"
)

type VictorOpsConfig struct {
	URL         string `json:"url,omitempty" yaml:"url,omitempty"`
	MessageType string `json:"messageType,omitempty" yaml:"messageType,omitempty"`
	Title       string `json:"title,omitempty" yaml:"title,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

func BuildVictorOpsConfig(fc receivers.FactoryConfig) (VictorOpsConfig, error) {
	settings := VictorOpsConfig{}
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
		settings.Title = templates.DefaultMessageTitleEmbed
	}
	if settings.Description == "" {
		settings.Description = templates.DefaultMessageEmbed
	}
	return settings, nil
}
