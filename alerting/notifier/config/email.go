package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/alerting/alerting/notifier/template"
)

type EmailSettings struct {
	SingleEmail bool
	Addresses   []string
	Message     string
	Subject     string
}

func BuildEmailSettings(fc FactoryConfig) (*EmailSettings, error) {
	type emailSettingsRaw struct {
		SingleEmail bool   `json:"singleEmail,omitempty"`
		Addresses   string `json:"addresses,omitempty"`
		Message     string `json:"message,omitempty"`
		Subject     string `json:"subject,omitempty"`
	}

	var settings emailSettingsRaw
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	if settings.Addresses == "" {
		return nil, errors.New("could not find addresses in settings")
	}
	// split addresses with a few different ways
	addresses := splitEmails(settings.Addresses)

	if settings.Subject == "" {
		settings.Subject = template.DefaultMessageTitleEmbed
	}

	return &EmailSettings{
		SingleEmail: settings.SingleEmail,
		Message:     settings.Message,
		Subject:     settings.Subject,
		Addresses:   addresses,
	}, nil
}

func splitEmails(emails string) []string {
	return strings.FieldsFunc(emails, func(r rune) bool {
		switch r {
		case ',', ';', '\n':
			return true
		}
		return false
	})
}
