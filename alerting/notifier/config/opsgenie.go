package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/alerting/alerting/notifier/template"
)

const (
	OpsgenieSendTags    = "tags"
	OpsgenieSendDetails = "details"
	OpsgenieSendBoth    = "both"

	DefaultOpsgenieAlertURL = "https://api.opsgenie.com/v2/alerts"
)

type OpsgenieSettings struct {
	APIKey           string
	APIUrl           string
	Message          string
	Description      string
	AutoClose        bool
	OverridePriority bool
	SendTagsAs       string
}

func BuildOpsgenieSettings(fc FactoryConfig) (*OpsgenieSettings, error) {
	type rawSettings struct {
		APIKey           string `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
		APIUrl           string `json:"apiUrl,omitempty" yaml:"apiUrl,omitempty"`
		Message          string `json:"message,omitempty" yaml:"message,omitempty"`
		Description      string `json:"description,omitempty" yaml:"description,omitempty"`
		AutoClose        *bool  `json:"autoClose,omitempty" yaml:"autoClose,omitempty"`
		OverridePriority *bool  `json:"overridePriority,omitempty" yaml:"overridePriority,omitempty"`
		SendTagsAs       string `json:"sendTagsAs,omitempty" yaml:"sendTagsAs,omitempty"`
	}

	raw := rawSettings{}
	err := json.Unmarshal(fc.Config.Settings, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	raw.APIKey = fc.DecryptFunc(context.Background(), fc.Config.SecureSettings, "apiKey", raw.APIKey)
	if raw.APIKey == "" {
		return nil, errors.New("could not find api key property in settings")
	}
	if raw.APIUrl == "" {
		raw.APIUrl = DefaultOpsgenieAlertURL
	}

	if strings.TrimSpace(raw.Message) == "" {
		raw.Message = template.DefaultMessageTitleEmbed
	}

	switch raw.SendTagsAs {
	case OpsgenieSendTags, OpsgenieSendDetails, OpsgenieSendBoth:
	case "":
		raw.SendTagsAs = OpsgenieSendTags
	default:
		return nil, fmt.Errorf("invalid value for sendTagsAs: %q", raw.SendTagsAs)
	}

	if raw.AutoClose == nil {
		autoClose := true
		raw.AutoClose = &autoClose
	}
	if raw.OverridePriority == nil {
		overridePriority := true
		raw.OverridePriority = &overridePriority
	}

	return &OpsgenieSettings{
		APIKey:           raw.APIKey,
		APIUrl:           raw.APIUrl,
		Message:          raw.Message,
		Description:      raw.Description,
		AutoClose:        *raw.AutoClose,
		OverridePriority: *raw.OverridePriority,
		SendTagsAs:       raw.SendTagsAs,
	}, nil
}
