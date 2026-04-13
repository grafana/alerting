package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/templates"
)

const Version = schema.V1

const (
	MessageTypeText   = "m.text"
	MessageTypeNotice = "m.notice"
)

type Config struct {
	HomeserverURL string `json:"homeserverUrl,omitempty" yaml:"homeserverUrl,omitempty"`
	AccessToken   string `json:"accessToken,omitempty" yaml:"accessToken,omitempty"`
	RoomID        string `json:"roomId,omitempty" yaml:"roomId,omitempty"`
	MessageType   string `json:"messageType,omitempty" yaml:"messageType,omitempty"`
	Message       string `json:"message,omitempty" yaml:"message,omitempty"`
	Title         string `json:"title,omitempty" yaml:"title,omitempty"`
}

func NewConfig(jsonData json.RawMessage, decryptFn receivers.DecryptFunc) (Config, error) {
	var settings Config
	if err := json.Unmarshal(jsonData, &settings); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	settings.HomeserverURL = strings.TrimRight(strings.TrimSpace(settings.HomeserverURL), "/")
	if settings.HomeserverURL == "" {
		return Config{}, errors.New("homeserver URL must be specified")
	}
	parsed, err := url.Parse(settings.HomeserverURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return Config{}, fmt.Errorf("invalid homeserver URL %q", settings.HomeserverURL)
	}
	settings.HomeserverURL = parsed.String()

	settings.AccessToken = decryptFn.Get("accessToken", settings.AccessToken)
	if settings.AccessToken == "" {
		return Config{}, errors.New("access token must be specified")
	}

	settings.RoomID = strings.TrimSpace(settings.RoomID)
	if settings.RoomID == "" {
		return Config{}, errors.New("room ID must be specified")
	}
	if !strings.HasPrefix(settings.RoomID, "!") || !strings.Contains(settings.RoomID, ":") {
		return Config{}, fmt.Errorf("room ID must be an internal room ID like \"!abc:example.com\", got %q", settings.RoomID)
	}

	switch settings.MessageType {
	case "":
		settings.MessageType = MessageTypeText
	case MessageTypeText, MessageTypeNotice:
	default:
		return Config{}, fmt.Errorf("invalid message type %q, must be %q or %q", settings.MessageType, MessageTypeText, MessageTypeNotice)
	}

	if settings.Message == "" {
		settings.Message = templates.DefaultMessageEmbed
	}
	if settings.Title == "" {
		settings.Title = templates.DefaultMessageTitleEmbed
	}
	return settings, nil
}

var Schema = schema.NewIntegrationSchemaVersion(schema.IntegrationSchemaVersion{
	Version:   Version,
	CanCreate: true,
	Options: []schema.Field{
		{
			Label:        "Homeserver URL",
			Element:      schema.ElementTypeInput,
			InputType:    schema.InputTypeText,
			Description:  "URL of the Matrix homeserver, e.g. https://matrix.example.com",
			Placeholder:  "https://matrix.example.com",
			PropertyName: "homeserverUrl",
			Required:     true,
		},
		{
			Label:        "Access token",
			Element:      schema.ElementTypeInput,
			InputType:    schema.InputTypeText,
			Description:  "Access token of the bot account that will post to the room",
			PropertyName: "accessToken",
			Required:     true,
			Secure:       true,
		},
		{
			Label:        "Room ID",
			Element:      schema.ElementTypeInput,
			InputType:    schema.InputTypeText,
			Description:  "Internal Matrix room ID. Must start with \"!\". Aliases like #room:server are not supported.",
			Placeholder:  "!abcdef:example.com",
			PropertyName: "roomId",
			Required:     true,
		},
		{
			Label:   "Message type",
			Element: schema.ElementTypeSelect,
			SelectOptions: []schema.SelectOption{
				{Value: MessageTypeText, Label: "Text (m.text)"},
				{Value: MessageTypeNotice, Label: "Notice (m.notice)"},
			},
			Description:  "Matrix event msgtype. m.notice marks the event as bot-generated so other bots will not reply to it.",
			PropertyName: "messageType",
		},
		{
			Label:        "Title",
			Element:      schema.ElementTypeInput,
			InputType:    schema.InputTypeText,
			Description:  "Templated title of the Matrix message",
			Placeholder:  templates.DefaultMessageTitleEmbed,
			PropertyName: "title",
		},
		{
			Label:        "Message",
			Element:      schema.ElementTypeTextArea,
			Description:  "Templated body of the Matrix message",
			Placeholder:  templates.DefaultMessageEmbed,
			PropertyName: "message",
		},
	},
})
