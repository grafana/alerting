package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/grafana/alerting/alerting/notifier/template"
)

type PushoverConfig struct {
	UserKey          string
	APIToken         string
	AlertingPriority int64
	OkPriority       int64
	Retry            int64
	Expire           int64
	Device           string
	AlertingSound    string
	OkSound          string
	Upload           bool
	Title            string
	Message          string
}

func BuildPushoverConfig(fc FactoryConfig) (PushoverConfig, error) {
	settings := PushoverConfig{}
	rawSettings := struct {
		UserKey          string      `json:"userKey,omitempty" yaml:"userKey,omitempty"`
		APIToken         string      `json:"apiToken,omitempty" yaml:"apiToken,omitempty"`
		AlertingPriority json.Number `json:"priority,omitempty" yaml:"priority,omitempty"`
		OKPriority       json.Number `json:"okPriority,omitempty" yaml:"okPriority,omitempty"`
		Retry            json.Number `json:"retry,omitempty" yaml:"retry,omitempty"`
		Expire           json.Number `json:"expire,omitempty" yaml:"expire,omitempty"`
		Device           string      `json:"device,omitempty" yaml:"device,omitempty"`
		AlertingSound    string      `json:"sound,omitempty" yaml:"sound,omitempty"`
		OKSound          string      `json:"okSound,omitempty" yaml:"okSound,omitempty"`
		Upload           *bool       `json:"uploadImage,omitempty" yaml:"uploadImage,omitempty"`
		Title            string      `json:"title,omitempty" yaml:"title,omitempty"`
		Message          string      `json:"message,omitempty" yaml:"message,omitempty"`
	}{}

	err := json.Unmarshal(fc.Config.Settings, &rawSettings)
	if err != nil {
		return settings, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	settings.UserKey = fc.DecryptFunc(context.Background(), fc.Config.SecureSettings, "userKey", rawSettings.UserKey)
	if settings.UserKey == "" {
		return settings, errors.New("user key not found")
	}
	settings.APIToken = fc.DecryptFunc(context.Background(), fc.Config.SecureSettings, "apiToken", rawSettings.APIToken)
	if settings.APIToken == "" {
		return settings, errors.New("API token not found")
	}
	if rawSettings.AlertingPriority != "" {
		settings.AlertingPriority, err = rawSettings.AlertingPriority.Int64()
		if err != nil {
			return settings, fmt.Errorf("failed to convert alerting priority to integer: %w", err)
		}
	}

	if rawSettings.OKPriority != "" {
		settings.OkPriority, err = rawSettings.OKPriority.Int64()
		if err != nil {
			return settings, fmt.Errorf("failed to convert OK priority to integer: %w", err)
		}
	}

	settings.Retry, _ = rawSettings.Retry.Int64()
	settings.Expire, _ = rawSettings.Expire.Int64()

	settings.Device = rawSettings.Device
	settings.AlertingSound = rawSettings.AlertingSound
	settings.OkSound = rawSettings.OKSound

	if rawSettings.Upload == nil || *rawSettings.Upload {
		settings.Upload = true
	}

	settings.Message = rawSettings.Message
	if settings.Message == "" {
		settings.Message = template.DefaultMessageEmbed
	}

	settings.Title = rawSettings.Title
	if settings.Title == "" {
		settings.Title = template.DefaultMessageTitleEmbed
	}

	return settings, nil
}
