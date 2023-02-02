package line

import (
	"context"
	"fmt"
	"net/url"
	"path"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	template2 "github.com/grafana/alerting/templates"
)

var (
	NotifyURL string = "https://notify-api.line.me/api/notify"
)

// Notifier is responsible for sending
// alert notifications to LINE.
type Notifier struct {
	*receivers.Base
	log      logging.Logger
	ns       receivers.WebhookSender
	tmpl     *template.Template
	settings *Config
}

// New is the constructor for the LINE notifier
func New(fc receivers.FactoryConfig) (*Notifier, error) {
	settings, err := ValidateConfig(fc)
	if err != nil {
		return nil, err
	}

	return &Notifier{
		Base:     receivers.NewBase(fc.Config),
		log:      fc.Logger,
		ns:       fc.NotificationService,
		tmpl:     fc.Template,
		settings: settings,
	}, nil
}

// Notify send an alert notification to LINE
func (ln *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	ln.log.Debug("executing line notification", "notification", ln.Name)

	body := ln.buildMessage(ctx, as...)

	form := url.Values{}
	form.Add("message", body)

	cmd := &receivers.SendWebhookSettings{
		URL:        NotifyURL,
		HTTPMethod: "POST",
		HTTPHeader: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", ln.settings.Token),
			"Content-Type":  "application/x-www-form-urlencoded;charset=UTF-8",
		},
		Body: form.Encode(),
	}

	if err := ln.ns.SendWebhook(ctx, cmd); err != nil {
		ln.log.Error("failed to send notification to LINE", "error", err, "body", body)
		return false, err
	}

	return true, nil
}

func (ln *Notifier) SendResolved() bool {
	return !ln.GetDisableResolveMessage()
}

func (ln *Notifier) buildMessage(ctx context.Context, as ...*types.Alert) string {
	ruleURL := path.Join(ln.tmpl.ExternalURL.String(), "/alerting/list")

	var tmplErr error
	tmpl, _ := template2.TmplText(ctx, ln.tmpl, as, ln.log, &tmplErr)

	body := fmt.Sprintf(
		"%s\n%s\n\n%s",
		tmpl(ln.settings.Title),
		ruleURL,
		tmpl(ln.settings.Description),
	)
	if tmplErr != nil {
		ln.log.Warn("failed to template Line message", "error", tmplErr.Error())
	}
	return body
}
