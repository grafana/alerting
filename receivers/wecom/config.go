package wecom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

var weComEndpoint = "https://qyapi.weixin.qq.com"

const DefaultWeComChannelType = "groupRobot"
const DefaultWeComMsgType = "markdown"
const DefaultWeComToUser = "@all"

type WeComMsgType string

const WeComMsgTypeMarkdown WeComMsgType = "markdown" // use these in available_receivers.go too
const WeComMsgTypeText WeComMsgType = "text"

// IsValid checks wecom message type
func (mt WeComMsgType) IsValid() bool {
	return mt == WeComMsgTypeMarkdown || mt == WeComMsgTypeText
}

type WecomConfig struct {
	Channel     string       `json:"-" yaml:"-"`
	EndpointURL string       `json:"endpointUrl,omitempty" yaml:"endpointUrl,omitempty"`
	URL         string       `json:"url" yaml:"url"`
	AgentID     string       `json:"agent_id,omitempty" yaml:"agent_id,omitempty"`
	CorpID      string       `json:"corp_id,omitempty" yaml:"corp_id,omitempty"`
	Secret      string       `json:"secret,omitempty" yaml:"secret,omitempty"`
	MsgType     WeComMsgType `json:"msgtype,omitempty" yaml:"msgtype,omitempty"`
	Message     string       `json:"message,omitempty" yaml:"message,omitempty"`
	Title       string       `json:"title,omitempty" yaml:"title,omitempty"`
	ToUser      string       `json:"touser,omitempty" yaml:"touser,omitempty"`
}

func BuildWecomConfig(factoryConfig receivers.FactoryConfig) (WecomConfig, error) {
	var settings = WecomConfig{
		Channel: DefaultWeComChannelType,
	}

	err := json.Unmarshal(factoryConfig.Config.Settings, &settings)
	if err != nil {
		return settings, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if len(settings.EndpointURL) == 0 {
		settings.EndpointURL = weComEndpoint
	}

	if !settings.MsgType.IsValid() {
		settings.MsgType = DefaultWeComMsgType
	}

	if len(settings.Message) == 0 {
		settings.Message = templates.DefaultMessageEmbed
	}
	if len(settings.Title) == 0 {
		settings.Title = templates.DefaultMessageTitleEmbed
	}
	if len(settings.ToUser) == 0 {
		settings.ToUser = DefaultWeComToUser
	}

	settings.URL = factoryConfig.DecryptFunc(context.Background(), factoryConfig.Config.SecureSettings, "url", settings.URL)
	settings.Secret = factoryConfig.DecryptFunc(context.Background(), factoryConfig.Config.SecureSettings, "secret", settings.Secret)

	if len(settings.URL) == 0 && len(settings.Secret) == 0 {
		return settings, errors.New("either url or secret is required")
	}

	if len(settings.URL) == 0 {
		settings.Channel = "apiapp"
		if len(settings.AgentID) == 0 {
			return settings, errors.New("could not find AgentID in settings")
		}
		if len(settings.CorpID) == 0 {
			return settings, errors.New("could not find CorpID in settings")
		}
	}

	return settings, nil
}
