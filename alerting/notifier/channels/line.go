package channels

import (
	"context"
	"fmt"
	"net/url"
	"path"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/alerting/log"
	"github.com/grafana/alerting/alerting/notifier/config"
	"github.com/grafana/alerting/alerting/notifier/sender"
	template2 "github.com/grafana/alerting/alerting/notifier/template"
)

var (
	LineNotifyURL string = "https://notify-api.line.me/api/notify"
)

// LineNotifier is responsible for sending
// alert notifications to LINE.
type LineNotifier struct {
	*Base
	log      log.Logger
	ns       sender.WebhookSender
	tmpl     *template.Template
	settings *config.LineSettings
}

func LineFactory(fc config.FactoryConfig) (NotificationChannel, error) {
	n, err := newLineNotifier(fc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return n, nil
}

// newLineNotifier is the constructor for the LINE notifier
func newLineNotifier(fc config.FactoryConfig) (*LineNotifier, error) {
	settings, err := config.BuildLineSettings(fc)
	if err != nil {
		return nil, err
	}

	return &LineNotifier{
		Base:     NewBase(fc.Config),
		log:      fc.Logger,
		ns:       fc.NotificationService,
		tmpl:     fc.Template,
		settings: settings,
	}, nil
}

// Notify send an alert notification to LINE
func (ln *LineNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	ln.log.Debug("executing line notification", "notification", ln.Name)

	body := ln.buildMessage(ctx, as...)

	form := url.Values{}
	form.Add("message", body)

	cmd := &sender.SendWebhookSettings{
		URL:        LineNotifyURL,
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

func (ln *LineNotifier) SendResolved() bool {
	return !ln.GetDisableResolveMessage()
}

func (ln *LineNotifier) buildMessage(ctx context.Context, as ...*types.Alert) string {
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
