package channels

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/alerting/log"
	"github.com/grafana/alerting/alerting/notifier/config"
	"github.com/grafana/alerting/alerting/notifier/images"
	"github.com/grafana/alerting/alerting/notifier/sender"
	template2 "github.com/grafana/alerting/alerting/notifier/template"
)

const (
	pushoverMaxFileSize = 1 << 21 // 2MB
	// https://pushover.net/api#limits - 250 characters or runes.
	pushoverMaxTitleLenRunes = 250
	// https://pushover.net/api#limits - 1024 characters or runes.
	pushoverMaxMessageLenRunes = 1024
	// https://pushover.net/api#limits - 512 characters or runes.
	pushoverMaxURLLenRunes = 512
)

var (
	PushoverEndpoint = "https://api.pushover.net/1/messages.json"
)

// PushoverNotifier is responsible for sending
// alert notifications to Pushover
type PushoverNotifier struct {
	*Base
	tmpl     *template.Template
	log      log.Logger
	images   images.ImageStore
	ns       sender.WebhookSender
	settings config.PushoverSettings
}

func PushoverFactory(fc config.FactoryConfig) (NotificationChannel, error) {
	notifier, err := NewPushoverNotifier(fc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return notifier, nil
}

// NewSlackNotifier is the constructor for the Slack notifier
func NewPushoverNotifier(fc config.FactoryConfig) (*PushoverNotifier, error) {
	settings, err := config.BuildPushoverSettings(fc)
	if err != nil {
		return nil, err
	}
	return &PushoverNotifier{
		Base:     NewBase(fc.Config),
		tmpl:     fc.Template,
		log:      fc.Logger,
		images:   fc.ImageStore,
		ns:       fc.NotificationService,
		settings: settings,
	}, nil
}

// Notify sends an alert notification to Slack.
func (pn *PushoverNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	headers, uploadBody, err := pn.genPushoverBody(ctx, as...)
	if err != nil {
		pn.log.Error("Failed to generate body for pushover", "error", err)
		return false, err
	}

	cmd := &sender.SendWebhookSettings{
		URL:        PushoverEndpoint,
		HTTPMethod: "POST",
		HTTPHeader: headers,
		Body:       uploadBody.String(),
	}

	if err := pn.ns.SendWebhook(ctx, cmd); err != nil {
		pn.log.Error("failed to send pushover notification", "error", err, "webhook", pn.Name)
		return false, err
	}

	return true, nil
}
func (pn *PushoverNotifier) SendResolved() bool {
	return !pn.GetDisableResolveMessage()
}

func (pn *PushoverNotifier) genPushoverBody(ctx context.Context, as ...*types.Alert) (map[string]string, bytes.Buffer, error) {
	key, err := notify.ExtractGroupKey(ctx)
	if err != nil {
		return nil, bytes.Buffer{}, err
	}

	b := bytes.Buffer{}
	w := multipart.NewWriter(&b)

	// tests use a non-random boundary separator
	if boundary := GetBoundary(); boundary != "" {
		err := w.SetBoundary(boundary)
		if err != nil {
			return nil, b, err
		}
	}

	var tmplErr error
	tmpl, _ := template2.TmplText(ctx, pn.tmpl, as, pn.log, &tmplErr)

	if err := w.WriteField("user", tmpl(pn.settings.UserKey)); err != nil {
		return nil, b, fmt.Errorf("failed to write the user: %w", err)
	}

	if err := w.WriteField("token", pn.settings.ApiToken); err != nil {
		return nil, b, fmt.Errorf("failed to write the token: %w", err)
	}

	title, truncated := TruncateInRunes(tmpl(pn.settings.Title), pushoverMaxTitleLenRunes)
	if truncated {
		pn.log.Warn("Truncated title", "incident", key, "max_runes", pushoverMaxTitleLenRunes)
	}
	message := tmpl(pn.settings.Message)
	message, truncated = TruncateInRunes(message, pushoverMaxMessageLenRunes)
	if truncated {
		pn.log.Warn("Truncated message", "incident", key, "max_runes", pushoverMaxMessageLenRunes)
	}
	message = strings.TrimSpace(message)
	if message == "" {
		// Pushover rejects empty messages.
		message = "(no details)"
	}

	supplementaryURL := joinURLPath(pn.tmpl.ExternalURL.String(), "/alerting/list", pn.log)
	supplementaryURL, truncated = TruncateInRunes(supplementaryURL, pushoverMaxURLLenRunes)
	if truncated {
		pn.log.Warn("Truncated URL", "incident", key, "max_runes", pushoverMaxURLLenRunes)
	}

	status := types.Alerts(as...).Status()
	priority := pn.settings.AlertingPriority
	if status == model.AlertResolved {
		priority = pn.settings.OkPriority
	}
	if err := w.WriteField("priority", strconv.FormatInt(priority, 10)); err != nil {
		return nil, b, fmt.Errorf("failed to write the priority: %w", err)
	}

	if priority == 2 {
		if err := w.WriteField("retry", strconv.FormatInt(pn.settings.Retry, 10)); err != nil {
			return nil, b, fmt.Errorf("failed to write retry: %w", err)
		}

		if err := w.WriteField("expire", strconv.FormatInt(pn.settings.Expire, 10)); err != nil {
			return nil, b, fmt.Errorf("failed to write expire: %w", err)
		}
	}

	if pn.settings.Device != "" {
		if err := w.WriteField("device", tmpl(pn.settings.Device)); err != nil {
			return nil, b, fmt.Errorf("failed to write the device: %w", err)
		}
	}

	if err := w.WriteField("title", title); err != nil {
		return nil, b, fmt.Errorf("failed to write the title: %w", err)
	}

	if err := w.WriteField("url", supplementaryURL); err != nil {
		return nil, b, fmt.Errorf("failed to write the URL: %w", err)
	}

	if err := w.WriteField("url_title", "Show alert rule"); err != nil {
		return nil, b, fmt.Errorf("failed to write the URL title: %w", err)
	}

	if err := w.WriteField("message", message); err != nil {
		return nil, b, fmt.Errorf("failed write the message: %w", err)
	}

	pn.writeImageParts(ctx, w, as...)

	var sound string
	if status == model.AlertResolved {
		sound = tmpl(pn.settings.OkSound)
	} else {
		sound = tmpl(pn.settings.AlertingSound)
	}
	if sound != "default" {
		if err := w.WriteField("sound", sound); err != nil {
			return nil, b, fmt.Errorf("failed to write the sound: %w", err)
		}
	}

	// Mark the message as HTML
	if err := w.WriteField("html", "1"); err != nil {
		return nil, b, fmt.Errorf("failed to mark the message as HTML: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, b, fmt.Errorf("failed to close the multipart request: %w", err)
	}

	if tmplErr != nil {
		pn.log.Warn("failed to template pushover message", "error", tmplErr.Error())
	}

	headers := map[string]string{
		"Content-Type": w.FormDataContentType(),
	}

	return headers, b, nil
}

func (pn *PushoverNotifier) writeImageParts(ctx context.Context, w *multipart.Writer, as ...*types.Alert) {
	// Pushover supports at most one image attachment with a maximum size of pushoverMaxFileSize.
	// If the image is larger than pushoverMaxFileSize then return an error.
	_ = withStoredImages(ctx, pn.log, pn.images, func(index int, image images.Image) error {
		f, err := os.Open(image.Path)
		if err != nil {
			return fmt.Errorf("failed to open the image: %w", err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				pn.log.Error("failed to close the image", "file", image.Path)
			}
		}()

		fileInfo, err := f.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat the image: %w", err)
		}

		if fileInfo.Size() > pushoverMaxFileSize {
			return fmt.Errorf("image would exceeded maximum file size: %d", fileInfo.Size())
		}

		fw, err := w.CreateFormFile("attachment", image.Path)
		if err != nil {
			return fmt.Errorf("failed to create form file for the image: %w", err)
		}

		if _, err = io.Copy(fw, f); err != nil {
			return fmt.Errorf("failed to copy the image to the form file: %w", err)
		}

		return images.ErrImagesDone
	}, as...)
}
