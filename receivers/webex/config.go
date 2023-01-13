package webex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

const (
	DefaultWebexAPIURL = "https://webexapis.com/v1/messages"
)

// PLEASE do not touch these settings without taking a look at what we support as part of
// https://github.com/prometheus/alertmanager/blob/main/notify/webex/webex.go
// Currently, the Alerting team is unifying channels and (upstream) receivers - any discrepancy is detrimental to that.
type WebexConfig struct {
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	RoomID  string `json:"room_id,omitempty" yaml:"room_id,omitempty"`
	APIURL  string `json:"api_url,omitempty" yaml:"api_url,omitempty"`
	Token   string `json:"bot_token" yaml:"bot_token"`
}

// BuildWebexConfig is the constructor for the Webex notifier.
func BuildWebexConfig(factoryConfig receivers.FactoryConfig) (*WebexConfig, error) {
	settings := &WebexConfig{}
	err := json.Unmarshal(factoryConfig.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.APIURL == "" {
		settings.APIURL = DefaultWebexAPIURL
	}

	if settings.Message == "" {
		settings.Message = templates.DefaultMessageEmbed
	}

	settings.Token = factoryConfig.DecryptFunc(context.Background(), factoryConfig.Config.SecureSettings, "bot_token", settings.Token)

	u, err := url.Parse(settings.APIURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q", settings.APIURL)
	}
	settings.APIURL = u.String()

	return settings, err
}
