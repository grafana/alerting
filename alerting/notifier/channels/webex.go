package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/notify/webex"
	"github.com/prometheus/alertmanager/types"
	config2 "github.com/prometheus/common/config"
)

const webexAPIURL = "https://webexapis.com/v1/messages"

// PLEASE do not touch these settings without taking a look at what we support as part of
// https://github.com/prometheus/alertmanager/blob/main/notify/webex/webex.go
// Currently, the Alerting team is unifying channels and (upstream) receivers - any discrepancy is detrimental to that.
type webexSettings struct {
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	RoomID  string `json:"room_id,omitempty" yaml:"room_id,omitempty"`
	APIURL  string `json:"api_url,omitempty" yaml:"api_url,omitempty"`
	Token   string `json:"bot_token" yaml:"bot_token"`
}

func buildWebexSettings(factoryConfig FactoryConfig) (*config.WebexConfig, error) {
	settings := &webexSettings{}
	err := factoryConfig.Config.unmarshalSettings(&settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.APIURL == "" {
		settings.APIURL = webexAPIURL
	}

	if settings.Message == "" {
		settings.Message = DefaultMessageEmbed
	}

	settings.Token = factoryConfig.DecryptFunc(context.Background(), factoryConfig.Config.SecureSettings, "bot_token", settings.Token)

	u, err := url.Parse(settings.APIURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q", settings.APIURL)
	}
	settings.APIURL = u.String()

	return &config.WebexConfig{
		NotifierConfig: config.NotifierConfig{
			VSendResolved: !factoryConfig.Config.DisableResolveMessage,
		},
		HTTPConfig: &config2.HTTPClientConfig{
			Authorization: &config2.Authorization{
				Type:        "Bearer",
				Credentials: config2.Secret(settings.Token),
			},
		},
		APIURL:  &config.URL{URL: u},
		Message: settings.Message,
		RoomID:  settings.RoomID,
	}, err
}

func WebexFactory(fc FactoryConfig) (*webex.Notifier, error) {
	notifier, err := buildWebexNotifier(fc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return notifier, nil
}

func withImages(imageStore ImageStore, logger Logger) webex.PreSendHookFunc {
	return func(ctx context.Context, payload webex.Payload, alerts []*types.Alert) (io.Reader, error) {
		extended := WebexMessage{
			Payload: payload,
			Files:   nil,
		}
		// Augment our Alert data with ImageURLs if available.
		_ = withStoredImages(ctx, logger, imageStore, func(index int, image Image) error {
			// Cisco Webex only supports a single image per request: https://developer.webex.com/docs/basics#message-attachments
			if image.HasURL() {
				extended.Files = append(extended.Files, image.URL)
				return ErrImagesDone
			}

			return nil
		}, alerts...)

		var buffer bytes.Buffer
		if err := json.NewEncoder(&buffer).Encode(payload); err != nil {
			return nil, err
		}
		return &buffer, nil
	}
}

// buildWebexSettings is the constructor for the Webex notifier.
func buildWebexNotifier(factoryConfig FactoryConfig) (*webex.Notifier, error) {
	settings, err := buildWebexSettings(factoryConfig)
	if err != nil {
		return nil, err
	}

	var options []config2.HTTPClientOption
	return webex.WithPreSendHook(webex.New(settings, factoryConfig.Template, factoryConfig.Logger, options...))(withImages(factoryConfig.ImageStore, factoryConfig.Logger))
}

// WebexMessage defines the JSON object to send to Webex endpoints.
type WebexMessage struct {
	webex.Payload
	Files []string `json:"files,omitempty"`
}
