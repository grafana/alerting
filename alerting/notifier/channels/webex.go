package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/alerting/log"
	"github.com/grafana/alerting/alerting/notifier/config"
	"github.com/grafana/alerting/alerting/notifier/images"
	"github.com/grafana/alerting/alerting/notifier/sender"
	template2 "github.com/grafana/alerting/alerting/notifier/template"
)

// WebexNotifier is responsible for sending alert notifications as webex messages.
type WebexNotifier struct {
	*Base
	ns       sender.WebhookSender
	log      log.Logger
	images   images.ImageStore
	tmpl     *template.Template
	orgID    int64
	settings *config.WebexSettings
}

func WebexFactory(fc config.FactoryConfig) (NotificationChannel, error) {
	notifier, err := buildWebexNotifier(fc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return notifier, nil
}

func buildWebexNotifier(factoryConfig config.FactoryConfig) (*WebexNotifier, error) {
	settings, err := config.BuildWebexSettings(factoryConfig)
	if err != nil {
		return nil, err
	}

	return &WebexNotifier{
		Base:     NewBase(factoryConfig.Config),
		orgID:    factoryConfig.Config.OrgID,
		log:      factoryConfig.Logger,
		ns:       factoryConfig.NotificationService,
		images:   factoryConfig.ImageStore,
		tmpl:     factoryConfig.Template,
		settings: settings,
	}, nil
}

// WebexMessage defines the JSON object to send to Webex endpoints.
type WebexMessage struct {
	RoomID  string   `json:"roomId,omitempty"`
	Message string   `json:"markdown"`
	Files   []string `json:"files,omitempty"`
}

// Notify implements the Notifier interface.
func (wn *WebexNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	var tmplErr error
	tmpl, data := template2.TmplText(ctx, wn.tmpl, as, wn.log, &tmplErr)

	message, truncated := TruncateInBytes(tmpl(wn.settings.Message), 4096)
	if truncated {
		wn.log.Warn("Webex message too long, truncating message", "OriginalMessage", wn.settings.Message)
	}

	if tmplErr != nil {
		wn.log.Warn("Failed to template webex message", "Error", tmplErr.Error())
		tmplErr = nil
	}

	msg := &WebexMessage{
		RoomID:  wn.settings.RoomID,
		Message: message,
		Files:   []string{},
	}

	// Augment our Alert data with ImageURLs if available.
	_ = withStoredImages(ctx, wn.log, wn.images, func(index int, image images.Image) error {
		// Cisco Webex only supports a single image per request: https://developer.webex.com/docs/basics#message-attachments
		if image.HasURL() {
			data.Alerts[index].ImageURL = image.URL
			msg.Files = append(msg.Files, image.URL)
			return images.ErrImagesDone
		}

		return nil
	}, as...)

	body, err := json.Marshal(msg)
	if err != nil {
		return false, err
	}

	parsedURL := tmpl(wn.settings.APIURL)
	if tmplErr != nil {
		return false, tmplErr
	}

	cmd := &sender.SendWebhookSettings{
		URL:        parsedURL,
		Body:       string(body),
		HTTPMethod: http.MethodPost,
	}

	if wn.settings.Token != "" {
		headers := make(map[string]string)
		headers["Authorization"] = fmt.Sprintf("Bearer %s", wn.settings.Token)
		cmd.HTTPHeader = headers
	}

	if err := wn.ns.SendWebhook(ctx, cmd); err != nil {
		return false, err
	}

	return true, nil
}

func (wn *WebexNotifier) SendResolved() bool {
	return !wn.GetDisableResolveMessage()
}
