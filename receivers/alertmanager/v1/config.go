package v1

import (
	"encoding/json"
	"errors"
	"net/url"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/schema"
)

const (
	Type    = schema.AlertManagerType
	Version = schema.V1
)

type Config struct {
	URLs     []*url.URL `json:"url,omitempty" yaml:"url,omitempty"`
	User     string     `json:"basicAuthUser,omitempty" yaml:"basicAuthUser,omitempty"`
	Password string     `json:"basicAuthPassword,omitempty" yaml:"basicAuthPassword,omitempty"`
}

func NewConfig(jsonData json.RawMessage, decryptFn receivers.DecryptFunc) (Config, error) {
	var settings Config
	if err := settings.UnmarshalJSON(jsonData); err != nil {
		return Config{}, err
	}
	if len(settings.URLs) == 0 {
		return Config{}, errors.New("could not find url property in settings")
	}
	settings.Password = decryptFn.Get("basicAuthPassword", settings.Password)
	return settings, nil
}

var Schema = schema.NewIntegrationSchemaVersion(schema.IntegrationSchemaVersion{
	Version:   Version,
	CanCreate: true,
	Options: []schema.Field{
		{
			Label:        "URL",
			Element:      schema.ElementTypeInput,
			InputType:    schema.InputTypeText,
			Placeholder:  "http://localhost:9093",
			PropertyName: "url",
			Required:     true,
			Protected:    true,
		},
		{
			Label:        "Basic Auth User",
			Element:      schema.ElementTypeInput,
			InputType:    schema.InputTypeText,
			PropertyName: "basicAuthUser",
		},
		{
			Label:        "Basic Auth Password",
			Element:      schema.ElementTypeInput,
			InputType:    schema.InputTypePassword,
			PropertyName: "basicAuthPassword",
			Secure:       true,
		},
	},
})

var Factory = receivers.NewIntegrationVersionFactory(
	Type, Version, NewConfig,
	func(cfg Config, m receivers.Metadata, opts receivers.NotifierOpts) (receivers.NotificationChannel, error) {
		return New(cfg, m, opts.Images, opts.Logger), nil
	},
)
