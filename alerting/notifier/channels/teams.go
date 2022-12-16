package channels

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/notifications"
)

// TeamsNotifier is responsible for sending
// alert notifications to Microsoft teams.
type TeamsNotifier struct {
	*Base
	URL     string
	Message string
	tmpl    *template.Template
	log     log.Logger
	ns      notifications.WebhookSender
}

type TeamsConfig struct {
	*NotificationChannelConfig
	URL     string
	Message string
}

func TeamsFactory(fc FactoryConfig) (NotificationChannel, error) {
	cfg, err := NewTeamsConfig(fc.Config)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return NewTeamsNotifier(cfg, fc.NotificationService, fc.Template), nil
}

func NewTeamsConfig(config *NotificationChannelConfig) (*TeamsConfig, error) {
	URL := config.Settings.Get("url").MustString()
	if URL == "" {
		return nil, errors.New("could not find url property in settings")
	}
	return &TeamsConfig{
		NotificationChannelConfig: config,
		URL:                       URL,
		Message:                   config.Settings.Get("message").MustString(`{{ template "teams.default.message" .}}`),
	}, nil
}

// NewTeamsNotifier is the constructor for Teams notifier.
func NewTeamsNotifier(config *TeamsConfig, ns notifications.WebhookSender, t *template.Template) *TeamsNotifier {
	return &TeamsNotifier{
		Base: NewBase(&models.AlertNotification{
			Uid:                   config.UID,
			Name:                  config.Name,
			Type:                  config.Type,
			DisableResolveMessage: config.DisableResolveMessage,
			Settings:              config.Settings,
		}),
		URL:     config.URL,
		Message: config.Message,
		log:     log.New("alerting.notifier.teams"),
		ns:      ns,
		tmpl:    t,
	}
}

// Notify send an alert notification to Microsoft teams.
func (tn *TeamsNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	var tmplErr error
	tmpl, _ := TmplText(ctx, tn.tmpl, as, tn.log, &tmplErr)

	ruleURL := joinUrlPath(tn.tmpl.ExternalURL.String(), "/alerting/list", tn.log)

	title := tmpl(DefaultMessageTitleEmbed)
	body := map[string]interface{}{
		"@type":    "MessageCard",
		"@context": "http://schema.org/extensions",
		// summary MUST not be empty or the webhook request fails
		// summary SHOULD contain some meaningful information, since it is used for mobile notifications
		"summary":    title,
		"title":      title,
		"themeColor": getAlertStatusColor(types.Alerts(as...).Status()),
		"sections": []map[string]interface{}{
			{
				"title": "Details",
				"text":  tmpl(tn.Message),
			},
		},
		"potentialAction": []map[string]interface{}{
			{
				"@context": "http://schema.org",
				"@type":    "OpenUri",
				"name":     "View Rule",
				"targets": []map[string]interface{}{
					{
						"os":  "default",
						"uri": ruleURL,
					},
				},
			},
		},
	}

	u := tmpl(tn.URL)
	if tmplErr != nil {
		tn.log.Warn("failed to template Teams message", "err", tmplErr.Error())
	}

	b, err := json.Marshal(&body)
	if err != nil {
		return false, errors.Wrap(err, "marshal json")
	}
	cmd := &models.SendWebhookSync{Url: u, Body: string(b)}

	if err := tn.ns.SendWebhookSync(ctx, cmd); err != nil {
		return false, errors.Wrap(err, "send notification to Teams")
	}

	return true, nil
}

func (tn *TeamsNotifier) SendResolved() bool {
	return !tn.GetDisableResolveMessage()
}
