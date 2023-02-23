package email

import (
	"context"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	template2 "github.com/grafana/alerting/templates"
)

// Notifier is responsible for sending
// alert notifications over email.
type Notifier struct {
	*receivers.Base
	log      logging.Logger
	ns       receivers.EmailSender
	images   images.ImageStore
	tmpl     *template.Template
	settings Config
}

func New(fc receivers.FactoryConfig) (*Notifier, error) {
	settings, err := NewConfig(fc.Config.Settings)
	if err != nil {
		return nil, err
	}
	return &Notifier{
		Base:     receivers.NewBase(fc.Config),
		log:      fc.Logger,
		ns:       fc.NotificationService,
		images:   fc.ImageStore,
		tmpl:     fc.Template,
		settings: settings,
	}, nil
}

// Notify sends the alert notification.
func (en *Notifier) Notify(ctx context.Context, alerts ...*types.Alert) (bool, error) {
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
	_ = images.WithStoredImages(ctx, en.log, en.images,
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

	cmd := &receivers.SendEmailSettings{
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

func (en *Notifier) SendResolved() bool {
	return !en.GetDisableResolveMessage()
}
