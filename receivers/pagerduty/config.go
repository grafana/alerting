package pagerduty

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

const (
	DefaultSeverity = "critical"
	DefaultClass    = "default"
	DefaultGroup    = "default"
	DefaultClient   = "Grafana"
)

var defaultCustomDetails = map[string]string{
	"firing":       `{{ template "__text_alert_list" .Alerts.Firing }}`,
	"resolved":     `{{ template "__text_alert_list" .Alerts.Resolved }}`,
	"num_firing":   `{{ .Alerts.Firing | len }}`,
	"num_resolved": `{{ .Alerts.Resolved | len }}`,
}

// mergeCustomDetails merges the default custom details with the user-defined ones.
// Default values get overwritten in case of duplicate keys.
func mergeCustomDetails(userDefinedCustomDetails map[string]string) map[string]string {
	mergedCustomDetails := make(map[string]string)
	for k, v := range defaultCustomDetails {
		mergedCustomDetails[k] = v
	}
	for k, v := range userDefinedCustomDetails {
		mergedCustomDetails[k] = v
	}
	return mergedCustomDetails
}

var getHostname = func() (string, error) {
	return os.Hostname()
}

type Config struct {
	Key           string            `json:"integrationKey,omitempty" yaml:"integrationKey,omitempty"`
	Severity      string            `json:"severity,omitempty" yaml:"severity,omitempty"`
	CustomDetails map[string]string `json:"custom_details,omitempty" yaml:"custom_details,omitempty"`
	Class         string            `json:"class,omitempty" yaml:"class,omitempty"`
	Component     string            `json:"component,omitempty" yaml:"component,omitempty"`
	Group         string            `json:"group,omitempty" yaml:"group,omitempty"`
	Summary       string            `json:"summary,omitempty" yaml:"summary,omitempty"`
	Source        string            `json:"source,omitempty" yaml:"source,omitempty"`
	Client        string            `json:"client,omitempty" yaml:"client,omitempty"`
	ClientURL     string            `json:"client_url,omitempty" yaml:"client_url,omitempty"`
}

func NewConfig(jsonData json.RawMessage, decryptFn receivers.DecryptFunc) (Config, error) {
	settings := Config{}
	err := json.Unmarshal(jsonData, &settings)
	if err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	settings.Key = decryptFn("integrationKey", settings.Key)
	if settings.Key == "" {
		return Config{}, errors.New("could not find integration key property in settings")
	}

	settings.CustomDetails = mergeCustomDetails(settings.CustomDetails)

	if settings.Severity == "" {
		settings.Severity = DefaultSeverity
	}
	if settings.Class == "" {
		settings.Class = DefaultClass
	}
	if settings.Component == "" {
		settings.Component = "Grafana"
	}
	if settings.Group == "" {
		settings.Group = DefaultGroup
	}
	if settings.Summary == "" {
		settings.Summary = templates.DefaultMessageTitleEmbed
	}
	if settings.Client == "" {
		settings.Client = DefaultClient
	}
	if settings.ClientURL == "" {
		settings.ClientURL = "{{ .ExternalURL }}"
	}
	if settings.Source == "" {
		source, err := getHostname()
		if err != nil {
			source = settings.Client
		}
		settings.Source = source
	}
	return settings, nil
}
