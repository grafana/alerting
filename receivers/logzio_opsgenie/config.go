package logzio_opsgenie

// LOGZ.IO GRAFANA CHANGE :: DEV-46341 - Add support for logzio opsgenie integration
import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
	"strings"
)

const (
	DefaultAlertsURL = "https://api.opsgenie.com/v1/json/logzio"
)

type Config struct {
	APIKey           string
	APIUrl           string
	Message          string
	Description      string
	AutoClose        bool
	OverridePriority bool
}

func NewConfig(jsonData json.RawMessage, decryptFn receivers.DecryptFunc) (Config, error) {
	type rawSettings struct {
		APIKey           string `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
		APIUrl           string `json:"apiUrl,omitempty" yaml:"apiUrl,omitempty"`
		Message          string `json:"message,omitempty" yaml:"message,omitempty"`
		Description      string `json:"description,omitempty" yaml:"description,omitempty"`
		AutoClose        *bool  `json:"autoClose,omitempty" yaml:"autoClose,omitempty"`
		OverridePriority *bool  `json:"overridePriority,omitempty" yaml:"overridePriority,omitempty"`
	}

	raw := rawSettings{}
	err := json.Unmarshal(jsonData, &raw)
	if err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	raw.APIKey = decryptFn("apiKey", raw.APIKey)
	if raw.APIKey == "" {
		return Config{}, errors.New("could not find api key property in settings")
	}
	if raw.APIUrl == "" {
		raw.APIUrl = DefaultAlertsURL
	}

	if strings.TrimSpace(raw.Message) == "" {
		raw.Message = templates.DefaultMessageTitleEmbed
	}

	if raw.AutoClose == nil {
		autoClose := true
		raw.AutoClose = &autoClose
	}
	if raw.OverridePriority == nil {
		overridePriority := true
		raw.OverridePriority = &overridePriority
	}

	return Config{
		APIKey:           raw.APIKey,
		APIUrl:           raw.APIUrl,
		Message:          raw.Message,
		Description:      raw.Description,
		AutoClose:        *raw.AutoClose,
		OverridePriority: *raw.OverridePriority,
	}, nil
}

// LOGZ.IO GRAFANA CHANGE :: end
