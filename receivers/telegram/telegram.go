package telegram

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	template2 "github.com/grafana/alerting/templates"
)

var (
	APIURL = "https://api.telegram.org/bot%s/%s"
)

// Telegram supports 4096 chars max - from https://limits.tginfo.me/en.
const telegramMaxMessageLenRunes = 4096

// Notifier is responsible for sending
// alert notifications to Telegram.
type Notifier struct {
	*receivers.Base
	log      logging.Logger
	images   images.ImageStore
	ns       receivers.WebhookSender
	tmpl     *template.Template
	settings Config
}

// New is the constructor for the Telegram notifier
func New(fc receivers.FactoryConfig) (*Notifier, error) {
	settings, err := BuildConfig(fc)
	if err != nil {
		return nil, err
	}
	return &Notifier{
		Base:     receivers.NewBase(fc.Config),
		tmpl:     fc.Template,
		log:      fc.Logger,
		images:   fc.ImageStore,
		ns:       fc.NotificationService,
		settings: settings,
	}, nil
}

// Notify send an alert notification to Telegram.
func (tn *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	// Create the cmd for sendMessage
	cmd, err := tn.newWebhookSyncCmd("sendMessage", func(w *multipart.Writer) error {
		msg, err := tn.buildTelegramMessage(ctx, as)
		if err != nil {
			return fmt.Errorf("failed to build message: %w", err)
		}
		for k, v := range msg {
			fw, err := w.CreateFormField(k)
			if err != nil {
				return fmt.Errorf("failed to create form field: %w", err)
			}
			if _, err := fw.Write([]byte(v)); err != nil {
				return fmt.Errorf("failed to write value: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to create telegram message: %w", err)
	}
	if err := tn.ns.SendWebhook(ctx, cmd); err != nil {
		return false, fmt.Errorf("failed to send telegram message: %w", err)
	}

	// Create the cmd to upload each image
	_ = images.WithStoredImages(ctx, tn.log, tn.images, func(index int, image images.Image) error {
		cmd, err = tn.newWebhookSyncCmd("sendPhoto", func(w *multipart.Writer) error {
			f, err := os.Open(image.Path)
			if err != nil {
				return fmt.Errorf("failed to open image: %w", err)
			}
			defer func() {
				if err := f.Close(); err != nil {
					tn.log.Warn("failed to close image", "error", err)
				}
			}()
			fw, err := w.CreateFormFile("photo", image.Path)
			if err != nil {
				return fmt.Errorf("failed to create form file: %w", err)
			}
			if _, err := io.Copy(fw, f); err != nil {
				return fmt.Errorf("failed to write to form file: %w", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to create image: %w", err)
		}
		if err := tn.ns.SendWebhook(ctx, cmd); err != nil {
			return fmt.Errorf("failed to upload image to telegram: %w", err)
		}
		return nil
	}, as...)

	return true, nil
}

func (tn *Notifier) buildTelegramMessage(ctx context.Context, as []*types.Alert) (map[string]string, error) {
	var tmplErr error
	defer func() {
		if tmplErr != nil {
			tn.log.Warn("failed to template Telegram message", "error", tmplErr)
		}
	}()

	tmpl, _ := template2.TmplText(ctx, tn.tmpl, as, tn.log, &tmplErr)
	// Telegram supports 4096 chars max
	messageText, truncated := receivers.TruncateInRunes(tmpl(tn.settings.Message), telegramMaxMessageLenRunes)
	if truncated {
		key, err := notify.ExtractGroupKey(ctx)
		if err != nil {
			return nil, err
		}
		tn.log.Warn("Truncated message", "alert", key, "max_runes", telegramMaxMessageLenRunes)
	}

	m := make(map[string]string)
	m["text"] = messageText
	if tn.settings.ParseMode != "" {
		m["parse_mode"] = tn.settings.ParseMode
	}
	if tn.settings.DisableNotifications {
		m["disable_notification"] = "true"
	}
	return m, nil
}

func (tn *Notifier) newWebhookSyncCmd(action string, fn func(writer *multipart.Writer) error) (*receivers.SendWebhookSettings, error) {
	b := bytes.Buffer{}
	w := multipart.NewWriter(&b)

	boundary := receivers.GetBoundary()
	if boundary != "" {
		if err := w.SetBoundary(boundary); err != nil {
			return nil, err
		}
	}

	fw, err := w.CreateFormField("chat_id")
	if err != nil {
		return nil, err
	}
	if _, err := fw.Write([]byte(tn.settings.ChatID)); err != nil {
		return nil, err
	}

	if err := fn(w); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart: %w", err)
	}

	cmd := &receivers.SendWebhookSettings{
		URL:        fmt.Sprintf(APIURL, tn.settings.BotToken, action),
		Body:       b.String(),
		HTTPMethod: "POST",
		HTTPHeader: map[string]string{
			"Content-Type": w.FormDataContentType(),
		},
	}
	return cmd, nil
}

func (tn *Notifier) SendResolved() bool {
	return !tn.GetDisableResolveMessage()
}
