package dooray

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/prometheus/alertmanager/types"
	"net/url"
	"path"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

// Notifier is responsible for sending
// alert notifications to Dooray.
type Notifier struct {
	*receivers.Base
	log      logging.Logger
	ns       receivers.WebhookSender
	tmpl     *templates.Template
	settings Config
}

func New(cfg Config, meta receivers.Metadata, template *templates.Template, sender receivers.WebhookSender, logger logging.Logger) *Notifier {
	return &Notifier{
		Base:     receivers.NewBase(meta),
		log:      logger,
		ns:       sender,
		tmpl:     template,
		settings: cfg,
	}
}

// Dooray WebHook Request structure
type doorayMessage struct {
	BotName      string       `json:"botName"`
	BotIconImage string       `json:"botIconImage"`
	Text         string       `json:"text"`
	Attachments  []attachment `json:"attachments,omitempty"`
}

type attachment struct {
	Title     string `json:"title"`
	TitleLink string `json:"titleLink"`
	Text      string `json:"text"`
	Color     string `json:"color"`
}

// Notify send a webhook notification to Dooray
func (dr *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	dr.log.Debug("executing Dooray notification", "notification", dr.Name)

	title, body, err := dr.buildMessage(ctx, as...)
	if err != nil {
		return false, fmt.Errorf("failed to build message: %w", err)
	}

	form := url.Values{}
	form.Add("message", body)

	req := &doorayMessage{
		BotName:      title,
		BotIconImage: dr.settings.IconURL,
		Text:         body,
	}

	var cmd *receivers.SendWebhookSettings

	if jsonReq, err := json.Marshal(req); err != nil {
		return false, err
	} else {
		cmd = &receivers.SendWebhookSettings{
			URL:        dr.settings.Url,
			HTTPMethod: "POST",
			HTTPHeader: map[string]string{
				"Content-Type": "application/json;charset=UTF-8",
			},
			Body: string(jsonReq),
		}
	}

	if err := dr.ns.SendWebhook(ctx, cmd); err != nil {
		dr.log.Error("failed to send notification to Dooray", "error", err, "body", body)
		return false, err
	}

	return true, nil
}

func (dr *Notifier) SendResolved() bool {
	return !dr.GetDisableResolveMessage()
}

func (dr *Notifier) buildMessage(ctx context.Context, as ...*types.Alert) (string, string, error) {
	ruleURL := path.Join(dr.tmpl.ExternalURL.String(), "/alerting/list")

	var tmplErr error
	tmpl, _ := templates.TmplText(ctx, dr.tmpl, as, dr.log, &tmplErr)
	if tmplErr != nil {
		dr.log.Warn("failed to build Dooray message", "error", tmplErr.Error())
	}

	title := tmpl(dr.settings.Title)
	body := fmt.Sprintf(
		"%s\n%s\n\n%s",
		title,
		ruleURL,
		tmpl(dr.settings.Description),
	)
	if tmplErr != nil {
		dr.log.Warn("failed to template Dooray message", "error", tmplErr.Error())
	}
	return title, body, nil
}
