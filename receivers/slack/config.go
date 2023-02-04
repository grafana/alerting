package slack

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

type Config struct {
	EndpointURL    string                          `json:"endpointUrl,omitempty" yaml:"endpointUrl,omitempty"`
	URL            receivers.Secret                `json:"url,omitempty" yaml:"url,omitempty"`
	Token          receivers.Secret                `json:"token,omitempty" yaml:"token,omitempty"`
	Recipient      string                          `json:"recipient,omitempty" yaml:"recipient,omitempty"`
	Text           string                          `json:"text,omitempty" yaml:"text,omitempty"`
	Title          string                          `json:"title,omitempty" yaml:"title,omitempty"`
	Username       string                          `json:"username,omitempty" yaml:"username,omitempty"`
	IconEmoji      string                          `json:"icon_emoji,omitempty" yaml:"icon_emoji,omitempty"`
	IconURL        string                          `json:"icon_url,omitempty" yaml:"icon_url,omitempty"`
	MentionChannel string                          `json:"mentionChannel,omitempty" yaml:"mentionChannel,omitempty"`
	MentionUsers   receivers.CommaSeparatedStrings `json:"mentionUsers,omitempty" yaml:"mentionUsers,omitempty"`
	MentionGroups  receivers.CommaSeparatedStrings `json:"mentionGroups,omitempty" yaml:"mentionGroups,omitempty"`
}

func ValidateConfig(factoryConfig receivers.FactoryConfig) (Config, error) {
	var settings Config
	err := factoryConfig.Marshaller.Unmarshal(factoryConfig.Config.Settings, &settings)
	if err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.EndpointURL == "" {
		settings.EndpointURL = APIURL
	}
	slackURL := string(settings.URL)
	if slackURL == "" {
		slackURL = settings.EndpointURL
	}

	apiURL, err := url.Parse(slackURL)
	if err != nil {
		return Config{}, fmt.Errorf("invalid URL %q", slackURL)
	}
	settings.URL = receivers.Secret(apiURL.String())

	settings.Recipient = strings.TrimSpace(settings.Recipient)
	if settings.Recipient == "" && string(settings.URL) == APIURL {
		return Config{}, errors.New("recipient must be specified when using the Slack chat API")
	}
	if settings.MentionChannel != "" && settings.MentionChannel != "here" && settings.MentionChannel != "channel" {
		return Config{}, fmt.Errorf("invalid value for mentionChannel: %q", settings.MentionChannel)
	}
	if settings.Token == "" && string(settings.URL) == APIURL {
		return Config{}, errors.New("token must be specified when using the Slack chat API")
	}
	if settings.Username == "" {
		settings.Username = "Grafana"
	}
	if settings.Text == "" {
		settings.Text = templates.DefaultMessageEmbed
	}
	if settings.Title == "" {
		settings.Title = templates.DefaultMessageTitleEmbed
	}

	return settings, nil
}
