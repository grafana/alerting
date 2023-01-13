package channels

import (
	"context"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/alerting/log"
	"github.com/grafana/alerting/alerting/notifier/config"
	"github.com/grafana/alerting/alerting/notifier/images"
	"github.com/grafana/alerting/alerting/notifier/sender"
	template2 "github.com/grafana/alerting/alerting/notifier/template"
)

// EmailNotifier is responsible for sending
// alert notifications over email.
type EmailNotifier struct {
	*Base
	log      log.Logger
	ns       sender.EmailSender
	images   images.ImageStore
	tmpl     *template.Template
	settings *config.EmailSettings
}

func EmailFactory(fc config.FactoryConfig) (NotificationChannel, error) {
	notifier, err := buildEmailNotifier(fc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return notifier, nil
}

func buildEmailNotifier(fc config.FactoryConfig) (*EmailNotifier, error) {
	settings, err := config.BuildEmailSettings(fc)
	if err != nil {
		return nil, err
	}
	return &EmailNotifier{
		Base:     NewBase(fc.Config),
		log:      fc.Logger,
		ns:       fc.NotificationService,
		images:   fc.ImageStore,
		tmpl:     fc.Template,
		settings: settings,
	}, nil
}

// Notify sends the alert notification.
func (en *EmailNotifier) Notify(ctx context.Context, alerts ...*types.Alert) (bool, error) {
	var tmplErr error
	tmpl, data := template2.TmplText(ctx, en.tmpl, alerts, en.log, &tmplErr)

	subject := tmpl(en.settings.Subject)
	alertPageURL := en.tmpl.ExternalURL.String()
	ruleURL := en.tmpl.ExternalURL.String()
	u, err := url.Parse(en.tmpl.ExternalURL.String())
	if err == nil {
		basePath := u.Path
		u.Path = path.Join(basePath, "/alerting/list")
		ruleURL = u.String()
		u.RawQuery = "alertState=firing&view=state"
		alertPageURL = u.String()
	} else {
		en.log.Debug("failed to parse external URL", "url", en.tmpl.ExternalURL.String(), "error", err.Error())
	}

	// Extend alerts data with images, if available.
	var embeddedFiles []string
	_ = withStoredImages(ctx, en.log, en.images,
		func(index int, image images.Image) error {
			if len(image.URL) != 0 {
				data.Alerts[index].ImageURL = image.URL
			} else if len(image.Path) != 0 {
				_, err := os.Stat(image.Path)
				if err == nil {
					data.Alerts[index].EmbeddedImage = filepath.Base(image.Path)
					embeddedFiles = append(embeddedFiles, image.Path)
				} else {
					en.log.Warn("failed to get image file for email attachment", "file", image.Path, "error", err)
				}
			}
			return nil
		}, alerts...)

	cmd := &sender.SendEmailSettings{
		Subject: subject,
		Data: map[string]interface{}{
			"Title":             subject,
			"Message":           tmpl(en.settings.Message),
			"Status":            data.Status,
			"Alerts":            data.Alerts,
			"GroupLabels":       data.GroupLabels,
			"CommonLabels":      data.CommonLabels,
			"CommonAnnotations": data.CommonAnnotations,
			"ExternalURL":       data.ExternalURL,
			"RuleUrl":           ruleURL,
			"AlertPageUrl":      alertPageURL,
		},
		EmbeddedFiles: embeddedFiles,
		To:            en.settings.Addresses,
		SingleEmail:   en.settings.SingleEmail,
		Template:      "ng_alert_notification",
	}

	if tmplErr != nil {
		en.log.Warn("failed to template email message", "error", tmplErr.Error())
	}

	if err := en.ns.SendEmail(ctx, cmd); err != nil {
		return false, err
	}

	return true, nil
}

func (en *EmailNotifier) SendResolved() bool {
	return !en.GetDisableResolveMessage()
}
