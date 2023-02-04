package webex

import (
	"fmt"
	"net/url"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

const (
	DefaultAPIURL = "https://webexapis.com/v1/messages"
)

// PLEASE do not touch these settings without taking a look at what we support as part of
// https://github.com/prometheus/alertmanager/blob/main/notify/webex/webex.go
// Currently, the Alerting team is unifying channels and (upstream) receivers - any discrepancy is detrimental to that.
type Config struct {
	Message string           `json:"message,omitempty" yaml:"message,omitempty"`
	RoomID  string           `json:"room_id,omitempty" yaml:"room_id,omitempty"`
	APIURL  string           `json:"api_url,omitempty" yaml:"api_url,omitempty"`
	Token   receivers.Secret `json:"bot_token" yaml:"bot_token"`
}

// ValidateConfig is the constructor for the Webex notifier.
func ValidateConfig(factoryConfig receivers.FactoryConfig) (*Config, error) {
	settings := &Config{}
	err := factoryConfig.Marshaller.Unmarshal(factoryConfig.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.APIURL == "" {
		settings.APIURL = DefaultAPIURL
	}

	if settings.Message == "" {
		settings.Message = templates.DefaultMessageEmbed
	}

	u, err := url.Parse(settings.APIURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q", settings.APIURL)
	}
	settings.APIURL = u.String()

	return settings, err
}
