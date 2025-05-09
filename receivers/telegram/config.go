package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

const DefaultTelegramParseMode = "HTML"

// SupportedParseMode is a map of all supported values for field `parse_mode`. https://core.telegram.org/bots/api#formatting-options.
// Keys are options accepted by Grafana API, values are options accepted by Telegram API
var SupportedParseMode = map[string]string{"Markdown": "Markdown", "MarkdownV2": "MarkdownV2", DefaultTelegramParseMode: "HTML", "None": ""}

type Config struct {
	BotToken              string `json:"bottoken,omitempty" yaml:"bottoken,omitempty"`
	ChatID                string `json:"chatid,omitempty" yaml:"chatid,omitempty"`
	MessageThreadID       string `json:"message_thread_id,omitempty" yaml:"message_thread_id,omitempty"`
	Message               string `json:"message,omitempty" yaml:"message,omitempty"`
	ParseMode             string `json:"parse_mode,omitempty" yaml:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty" yaml:"disable_web_page_preview,omitempty"`
	ProtectContent        bool   `json:"protect_content,omitempty" yaml:"protect_content,omitempty"`
	DisableNotifications  bool   `json:"disable_notifications,omitempty" yaml:"disable_notifications,omitempty"`
}

type unmarshalConfig struct {
	BotToken              string `json:"bottoken,omitempty" yaml:"bottoken,omitempty"`
	ChatID                any    `json:"chatid,omitempty" yaml:"chatid,omitempty"`
	MessageThreadID       any    `json:"message_thread_id,omitempty" yaml:"message_thread_id,omitempty"`
	Message               string `json:"message,omitempty" yaml:"message,omitempty"`
	ParseMode             string `json:"parse_mode,omitempty" yaml:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty" yaml:"disable_web_page_preview,omitempty"`
	ProtectContent        bool   `json:"protect_content,omitempty" yaml:"protect_content,omitempty"`
	DisableNotifications  bool   `json:"disable_notifications,omitempty" yaml:"disable_notifications,omitempty"`
}

func NewConfig(jsonData json.RawMessage, decryptFn receivers.DecryptFunc) (Config, error) {
	settings := Config{}

	unmarshaledConfig := unmarshalConfig{}
	err := json.Unmarshal(jsonData, &unmarshaledConfig)
	if err != nil {
		return settings, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	settings.BotToken = unmarshaledConfig.BotToken
	settings.Message = unmarshaledConfig.Message
	settings.ParseMode = unmarshaledConfig.ParseMode
	settings.DisableWebPagePreview = unmarshaledConfig.DisableWebPagePreview
	settings.ProtectContent = unmarshaledConfig.ProtectContent
	settings.DisableNotifications = unmarshaledConfig.DisableNotifications

	settings.BotToken = decryptFn("bottoken", settings.BotToken)
	if settings.BotToken == "" {
		return settings, errors.New("could not find Bot Token in settings")
	}

	switch chatID := unmarshaledConfig.ChatID.(type) {
	case string:
		settings.ChatID = chatID

	case float64:
		settings.ChatID = strconv.Itoa(int(chatID))

	case nil:
		return settings, errors.New("could not find Chat Id in settings")

	default:
		return settings, errors.New("chat id must be either a string or an int")
	}

	if settings.Message == "" {
		settings.Message = templates.DefaultMessageEmbed
	}

	var messageThreadID int
	switch id := unmarshaledConfig.MessageThreadID.(type) {
	case string:
		messageThreadID, err = strconv.Atoi(id)
		if err != nil {
			return settings, errors.New("message thread id must be an integer")
		}

	case float64:
		messageThreadID = int(id)

	case nil:

	default:
		return settings, errors.New("message thread id must be either a string or an int")
	}

	if messageThreadID != 0 {
		if messageThreadID != int(int32(messageThreadID)) {
			return settings, errors.New("message thread id must be an int32")
		}

		settings.MessageThreadID = strconv.Itoa(messageThreadID)
	}

	// if field is missing, then we fall back to the previous default: HTML
	if settings.ParseMode == "" {
		settings.ParseMode = DefaultTelegramParseMode
	}
	found := false
	for parseMode, value := range SupportedParseMode {
		if strings.EqualFold(settings.ParseMode, parseMode) {
			settings.ParseMode = value
			found = true
			break
		}
	}
	if !found {
		return settings, fmt.Errorf("unknown parse_mode, must be Markdown, MarkdownV2, HTML or None")
	}
	return settings, nil
}
