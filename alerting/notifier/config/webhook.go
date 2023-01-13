package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/grafana/alerting/alerting/notifier/template"
)

type WebhookConfig struct {
	URL        string
	HTTPMethod string
	MaxAlerts  int
	// Authorization Header.
	AuthorizationScheme      string
	AuthorizationCredentials string
	// HTTP Basic Authentication.
	User     string
	Password string

	Title   string
	Message string
}

func BuildWebhookConfig(factoryConfig FactoryConfig) (WebhookConfig, error) {
	settings := WebhookConfig{}
	rawSettings := struct {
		URL                      string      `json:"url,omitempty" yaml:"url,omitempty"`
		HTTPMethod               string      `json:"httpMethod,omitempty" yaml:"httpMethod,omitempty"`
		MaxAlerts                json.Number `json:"maxAlerts,omitempty" yaml:"maxAlerts,omitempty"`
		AuthorizationScheme      string      `json:"authorization_scheme,omitempty" yaml:"authorization_scheme,omitempty"`
		AuthorizationCredentials string      `json:"authorization_credentials,omitempty" yaml:"authorization_credentials,omitempty"`
		User                     string      `json:"username,omitempty" yaml:"username,omitempty"`
		Password                 string      `json:"password,omitempty" yaml:"password,omitempty"`
		Title                    string      `json:"title,omitempty" yaml:"title,omitempty"`
		Message                  string      `json:"message,omitempty" yaml:"message,omitempty"`
	}{}

	err := json.Unmarshal(factoryConfig.Config.Settings, &rawSettings)
	if err != nil {
		return settings, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	if rawSettings.URL == "" {
		return settings, errors.New("required field 'url' is not specified")
	}
	settings.URL = rawSettings.URL

	if rawSettings.HTTPMethod == "" {
		rawSettings.HTTPMethod = http.MethodPost
	}
	settings.HTTPMethod = rawSettings.HTTPMethod

	if rawSettings.MaxAlerts != "" {
		settings.MaxAlerts, _ = strconv.Atoi(rawSettings.MaxAlerts.String())
	}

	settings.User = factoryConfig.DecryptFunc(context.Background(), factoryConfig.Config.SecureSettings, "username", rawSettings.User)
	settings.Password = factoryConfig.DecryptFunc(context.Background(), factoryConfig.Config.SecureSettings, "password", rawSettings.Password)
	settings.AuthorizationCredentials = factoryConfig.DecryptFunc(context.Background(), factoryConfig.Config.SecureSettings, "authorization_scheme", rawSettings.AuthorizationCredentials)

	if settings.AuthorizationCredentials != "" && settings.AuthorizationScheme == "" {
		settings.AuthorizationScheme = "Bearer"
	}
	if settings.User != "" && settings.Password != "" && settings.AuthorizationScheme != "" && settings.AuthorizationCredentials != "" {
		return settings, errors.New("both HTTP Basic Authentication and Authorization Header are set, only 1 is permitted")
	}
	settings.Title = rawSettings.Title
	if settings.Title == "" {
		settings.Title = template.DefaultMessageTitleEmbed
	}
	settings.Message = rawSettings.Message
	if settings.Message == "" {
		settings.Message = template.DefaultMessageEmbed
	}
	return settings, err
}
