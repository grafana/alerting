package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"sync"

	"github.com/go-kit/log/level"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"

	"github.com/go-kit/log"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

var (
	// APIURL of where the notification payload is sent. It is public to be overridable in integration tests.
	APIURL = "https://api.telegram.org/bot%s/%s"
)

// Telegram supports 4096 chars max - from https://limits.tginfo.me/en.
const telegramMaxMessageLenRunes = 4096

// Notifier is responsible for sending
// alert notifications to Telegram.
// It uses two API endpoints
// - https://core.telegram.org/bots/api#sendphoto for sending images (only if alerts contain references to them)
// - https://core.telegram.org/bots/api#sendmessage for sending text message
type Notifier struct {
	*receivers.Base
	images        images.Provider
	ns            receivers.WebhookSender
	tmpl          *templates.Template
	settings      Config
	lastMessageID int
	mu            sync.RWMutex
}

// New is the constructor for the Telegram notifier
func New(cfg Config, meta receivers.Metadata, template *templates.Template, sender receivers.WebhookSender, images images.Provider, logger log.Logger) *Notifier {
	return &Notifier{
		Base:     receivers.NewBase(meta, logger),
		tmpl:     template,
		images:   images,
		ns:       sender,
		settings: cfg,
	}
}

// Notify send an alert notification to Telegram.
func (tn *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	l := tn.GetLogger(ctx)
	// Create the cmd for sendMessage
	cmd, err := tn.newWebhookSyncCmd("sendMessage", func(w *multipart.Writer) error {
		var tmplErr error
		tmpl, _ := templates.TmplText(ctx, tn.tmpl, as, l, &tmplErr)
		messageText, truncated := receivers.TruncateInRunes(tmpl(tn.settings.Message), telegramMaxMessageLenRunes)
		if tmplErr != nil {
			level.Warn(l).Log("msg", "failed to template Telegram message", "err", tmplErr)
		}
		if truncated {
			key, err := notify.ExtractGroupKey(ctx)
			if err != nil {
				return err
			}
			level.Warn(l).Log("msg", "Truncated message", "alert", key, "max_runes", telegramMaxMessageLenRunes)
		}

		if err := w.WriteField("chat_id", tn.settings.ChatID); err != nil {
			return fmt.Errorf("failed to write chat_id: %w", err)
		}
		if err := w.WriteField("text", messageText); err != nil {
			return fmt.Errorf("failed to write text: %w", err)
		}
		if tn.settings.DisableWebPagePreview {
			if err := w.WriteField("disable_web_page_preview", "true"); err != nil {
				return fmt.Errorf("failed to write disable_web_page_preview: %w", err)
			}
		}
		if tn.settings.ProtectContent {
			if err := w.WriteField("protect_content", "true"); err != nil {
				return fmt.Errorf("failed to write protect_content: %w", err)
			}
		}
		if tn.settings.DisableNotifications {
			if err := w.WriteField("disable_notification", "true"); err != nil {
				return fmt.Errorf("failed to write disable_notification: %w", err)
			}
		}
		if tn.settings.MessageThreadID != "" {
			if err := w.WriteField("message_thread_id", tn.settings.MessageThreadID); err != nil {
				return fmt.Errorf("failed to write message_thread_id: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to create telegram message: %w", err)
	}

	resp, err := tn.ns.SendWebhook(ctx, l, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to send telegram message: %w", err)
	}

	var messageID int
	if resp != nil && resp.StatusCode/100 == 2 && len(resp.Body) > 0 {
		if extractedID, err := tn.extractMessageID(resp.Body); err == nil {
			messageID = extractedID
			tn.SetLastMessageID(messageID)
		}
	}
	if messageID > 0 {
		level.Info(l).Log("msg", "Editing message to apply formatting", "message_id", messageID, "parse_mode", tn.settings.ParseMode)

		if err := tn.EditMessage(ctx, messageID, as...); err != nil {
			level.Warn(l).Log("msg", "Failed to edit message for formatting", "error", err)
		}
	} else {
		level.Info(l).Log("msg", "No message ID available for editing, message sent without formatting")
	}

	// Create the cmd to upload each image
	uploadedImages := make(map[string]struct{})
	_ = images.WithStoredImages(ctx, l, tn.images, func(_ int, image images.Image) error {
		if _, ok := uploadedImages[image.ID]; ok && image.ID != "" { // Do not deduplicate if ID is not specified.
			return nil
		}
		cmd, err = tn.newWebhookSyncCmd("sendPhoto", func(w *multipart.Writer) error {
			f, err := image.RawData(ctx)
			if err != nil {
				return fmt.Errorf("failed to open image: %w", err)
			}
			fw, err := w.CreateFormFile("photo", f.Name)
			if err != nil {
				return fmt.Errorf("failed to create form file: %w", err)
			}
			if _, err := fw.Write(f.Content); err != nil {
				return fmt.Errorf("failed to write to form file: %w", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to create image: %w", err)
		}
		if _, err := tn.ns.SendWebhook(ctx, l, cmd); err != nil {
			return fmt.Errorf("failed to upload image to telegram: %w", err)
		}
		uploadedImages[image.ID] = struct{}{}
		return nil
	}, as...)

	return true, nil
}

func (tn *Notifier) buildTelegramMessage(ctx context.Context, as []*types.Alert, l log.Logger) (map[string]string, error) {
	var tmplErr error
	defer func() {
		if tmplErr != nil {
			level.Warn(l).Log("msg", "failed to template Telegram message", "err", tmplErr)
		}
	}()

	tmpl, _ := templates.TmplText(ctx, tn.tmpl, as, l, &tmplErr)
	// Telegram supports 4096 chars max
	messageText, truncated := receivers.TruncateInRunes(tmpl(tn.settings.Message), telegramMaxMessageLenRunes)
	if truncated {
		key, err := notify.ExtractGroupKey(ctx)
		if err != nil {
			return nil, err
		}
		level.Warn(l).Log("msg", "Truncated message", "alert", key, "max_runes", telegramMaxMessageLenRunes)
	}

	m := make(map[string]string)
	m["text"] = messageText
	if tn.settings.ParseMode != "" {
		m["parse_mode"] = tn.settings.ParseMode
	}
	if tn.settings.DisableWebPagePreview {
		m["disable_web_page_preview"] = "true"
	}
	if tn.settings.ProtectContent {
		m["protect_content"] = "true"
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

	if err := w.WriteField("chat_id", tn.settings.ChatID); err != nil {
		return nil, err
	}
	if tn.settings.MessageThreadID != "" {
		if err := w.WriteField("message_thread_id", tn.settings.MessageThreadID); err != nil {
			return nil, err
		}
	}
	if tn.settings.DisableNotifications {
		if err := w.WriteField("disable_notification", "true"); err != nil {
			return nil, err
		}
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

func (tn *Notifier) newEditMessageCmd(messageID int, msg map[string]string, l log.Logger) (*receivers.SendWebhookSettings, error) {
	b := bytes.Buffer{}
	w := multipart.NewWriter(&b)

	boundary := receivers.GetBoundary()
	if boundary != "" {
		if err := w.SetBoundary(boundary); err != nil {
			return nil, err
		}
	}

	if err := w.WriteField("chat_id", tn.settings.ChatID); err != nil {
		return nil, err
	}
	if err := w.WriteField("message_id", fmt.Sprintf("%d", messageID)); err != nil {
		return nil, err
	}
	if err := w.WriteField("text", msg["text"]); err != nil {
		return nil, err
	}

	if parseMode := msg["parse_mode"]; parseMode != "" {
		if err := w.WriteField("parse_mode", parseMode); err != nil {
			return nil, err
		}
	}
	if disablePreview := msg["disable_web_page_preview"]; disablePreview == "true" {
		if err := w.WriteField("disable_web_page_preview", "true"); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart: %w", err)
	}

	cmd := &receivers.SendWebhookSettings{
		URL:        fmt.Sprintf(APIURL, tn.settings.BotToken, "editMessageText"),
		Body:       b.String(),
		HTTPMethod: "POST",
		HTTPHeader: map[string]string{
			"Content-Type": w.FormDataContentType(),
		},
	}
	return cmd, nil
}

func (tn *Notifier) GetLastMessageID() int {
	tn.mu.RLock()
	defer tn.mu.RUnlock()
	return tn.lastMessageID
}

func (tn *Notifier) SetLastMessageID(id int) {
	tn.mu.Lock()
	defer tn.mu.Unlock()
	tn.lastMessageID = id
}

func (tn *Notifier) extractMessageID(responseBody []byte) (int, error) {
	var response struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}

	if err := json.Unmarshal(responseBody, &response); err != nil {
		return 0, err
	}

	if !response.OK {
		return 0, fmt.Errorf("telegram API returned not ok")
	}

	return response.Result.MessageID, nil
}

func (tn *Notifier) SendResolved() bool {
	return !tn.GetDisableResolveMessage()
}

func (tn *Notifier) EditMessage(ctx context.Context, messageID int, as ...*types.Alert) error {
	l := tn.GetLogger(ctx)

	msg, err := tn.buildTelegramMessage(ctx, as, l)
	if err != nil {
		return fmt.Errorf("failed to build message: %w", err)
	}

	cmd, err := tn.newEditMessageCmd(messageID, msg, l)
	if err != nil {
		return fmt.Errorf("failed to create edit message command: %w", err)
	}

	resp, err := tn.ns.SendWebhook(ctx, l, cmd)
	if err != nil {
		return fmt.Errorf("failed to edit telegram message: %w", err)
	}
	if resp != nil && resp.StatusCode >= 400 {
		return fmt.Errorf("failed to edit telegram message: webhook response status %d %s, body: %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(resp.Body))
	}

	return nil
}
