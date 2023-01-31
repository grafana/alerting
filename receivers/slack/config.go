package slack

import (
	"github.com/grafana/alerting/receivers"
)

const FooterIconURL = "https://grafana.com/static/assets/img/fav32.png"

type Config struct {
	EndpointURL    string                          `json:"endpointUrl,omitempty" yaml:"endpointUrl,omitempty"`
	URL            string                          `json:"url,omitempty" yaml:"url,omitempty"`
	Token          string                          `json:"token,omitempty" yaml:"token,omitempty"`
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
