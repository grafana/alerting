package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/templates"
)

const (
	Type    = schema.EmailType
	Version = schema.V1
)

type Config struct {
	SingleEmail bool                       `json:"singleEmail,omitempty" yaml:"singleEmail,omitempty"`
	Addresses   receivers.DelimitedStrings `json:"addresses" yaml:"addresses"`
	Message     string                     `json:"message,omitempty" yaml:"message,omitempty"`
	Subject     string                     `json:"subject,omitempty" yaml:"subject,omitempty"`
}

func NewConfig(jsonData json.RawMessage, _ receivers.DecryptFunc) (Config, error) {
	var settings Config
	err := json.Unmarshal(jsonData, &settings)
	if err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	if settings.Addresses == nil {
		return Config{}, errors.New("could not find addresses in settings")
	}
	for i, address := range settings.Addresses {
		settings.Addresses[i] = strings.TrimSpace(address)
	}
	if settings.Subject == "" {
		settings.Subject = templates.DefaultMessageTitleEmbed
	}
	return settings, nil
}

var Schema = schema.NewIntegrationSchemaVersion(schema.IntegrationSchemaVersion{
	Version:   Version,
	CanCreate: true,
	Options: []schema.Field{
		{
			Label:        "Single email",
			Description:  "Send a single email to all recipients",
			Element:      schema.ElementTypeCheckbox,
			PropertyName: "singleEmail",
		},
		{
			Label:        "Addresses",
			Description:  "You can enter multiple email addresses using a \";\", \"\\n\" or  \",\" separator",
			Element:      schema.ElementTypeTextArea,
			PropertyName: "addresses",
			Required:     true,
		},
		{ // New in 8.0.
			Label:        "Message",
			Description:  "Optional message. You can use templates to customize this field. Using a custom message will replace the default message",
			Element:      schema.ElementTypeTextArea,
			PropertyName: "message",
			Placeholder:  templates.DefaultMessageEmbed,
		},
		{ // New in 9.0.
			Label:        "Subject",
			Element:      schema.ElementTypeTextArea,
			InputType:    schema.InputTypeText,
			Description:  "Optional subject. You can use templates to customize this field",
			PropertyName: "subject",
			Placeholder:  templates.DefaultMessageTitleEmbed,
		},
	},
})

var Factory = receivers.NewIntegrationVersionFactory(
	Type, Version, NewConfig,
	func(cfg Config, m receivers.Metadata, opts receivers.NotifierOpts) (receivers.NotificationChannel, error) {
		return New(cfg, m, opts.Template, opts.EmailSender, opts.Images, opts.Logger), nil
	},
)
