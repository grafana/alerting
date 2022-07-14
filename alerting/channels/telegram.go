package channels

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"

	gokit_log "github.com/go-kit/kit/log"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/alerting"
	old_notifiers "github.com/grafana/grafana/pkg/services/alerting/notifiers"
)

const (
	telegramAPIURL = "https://api.telegram.org/bot%s/sendMessage"
)

// TelegramNotifier is responsible for sending
// alert notifications to Telegram.
type TelegramNotifier struct {
	old_notifiers.NotifierBase
	BotToken string
	ChatID   string
	Message  string
	log      log.Logger
	tmpl     *template.Template
}

// NewTelegramNotifier is the constructor for the Telegram notifier
func NewTelegramNotifier(model *models.AlertNotification, t *template.Template) (*TelegramNotifier, error) {
	if model.Settings == nil {
		return nil, alerting.ValidationError{Reason: "No Settings Supplied"}
	}

	botToken := model.DecryptedValue("bottoken", model.Settings.Get("bottoken").MustString())
	chatID := model.Settings.Get("chatid").MustString()
	message := model.Settings.Get("message").MustString(`{{ template "default.message" . }}`)

	if botToken == "" {
		return nil, alerting.ValidationError{Reason: "Could not find Bot Token in settings"}
	}

	if chatID == "" {
		return nil, alerting.ValidationError{Reason: "Could not find Chat Id in settings"}
	}

	return &TelegramNotifier{
		NotifierBase: old_notifiers.NewNotifierBase(model),
		BotToken:     botToken,
		ChatID:       chatID,
		Message:      message,
		tmpl:         t,
		log:          log.New("alerting.notifier.telegram"),
	}, nil
}

// Notify send an alert notification to Telegram.
func (tn *TelegramNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	msg, err := tn.buildTelegramMessage(ctx, as)
	if err != nil {
		return false, err
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	defer func() {
		if err := w.Close(); err != nil {
			tn.log.Warn("Failed to close writer", "err", err)
		}
	}()

	for k, v := range msg {
		if err := writeField(w, k, v); err != nil {
			return false, err
		}
	}

	// We need to close it before using so that the last part
	// is added to the writer along with the boundary.
	if err := w.Close(); err != nil {
		return false, err
	}

	tn.log.Info("sending telegram notification", "chat_id", tn.ChatID)
	cmd := &models.SendWebhookSync{
		Url:        fmt.Sprintf(telegramAPIURL, tn.BotToken),
		Body:       body.String(),
		HttpMethod: "POST",
		HttpHeader: map[string]string{
			"Content-Type": w.FormDataContentType(),
		},
	}

	if err := bus.DispatchCtx(ctx, cmd); err != nil {
		tn.log.Error("Failed to send webhook", "error", err, "webhook", tn.Name)
		return false, err
	}

	return true, nil
}

func (tn *TelegramNotifier) buildTelegramMessage(ctx context.Context, as []*types.Alert) (map[string]string, error) {
	msg := map[string]string{}
	msg["chat_id"] = tn.ChatID
	msg["parse_mode"] = "html"

	data := notify.GetTemplateData(ctx, &template.Template{ExternalURL: tn.tmpl.ExternalURL}, as, gokit_log.NewNopLogger())
	var tmplErr error
	tmpl := notify.TmplText(tn.tmpl, data, &tmplErr)

	message := tmpl(tn.Message)
	if tmplErr != nil {
		return nil, tmplErr
	}

	msg["text"] = message

	return msg, nil
}

func writeField(w *multipart.Writer, name, value string) error {
	fw, err := w.CreateFormField(name)
	if err != nil {
		return err
	}
	if _, err := fw.Write([]byte(value)); err != nil {
		return err
	}
	return nil
}

func (tn *TelegramNotifier) SendResolved() bool {
	return !tn.GetDisableResolveMessage()
}
