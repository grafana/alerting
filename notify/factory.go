package notify

import (
	"fmt"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/alertmanager"
	"github.com/grafana/alerting/receivers/dinding"
	"github.com/grafana/alerting/receivers/discord"
	"github.com/grafana/alerting/receivers/email"
	"github.com/grafana/alerting/receivers/googlechat"
	"github.com/grafana/alerting/receivers/kafka"
	"github.com/grafana/alerting/receivers/line"
	"github.com/grafana/alerting/receivers/opsgenie"
	"github.com/grafana/alerting/receivers/pagerduty"
	"github.com/grafana/alerting/receivers/pushover"
	"github.com/grafana/alerting/receivers/sensugo"
	"github.com/grafana/alerting/receivers/slack"
	"github.com/grafana/alerting/receivers/teams"
	"github.com/grafana/alerting/receivers/telegram"
	"github.com/grafana/alerting/receivers/threema"
	"github.com/grafana/alerting/receivers/victorops"
	"github.com/grafana/alerting/receivers/webex"
	"github.com/grafana/alerting/receivers/webhook"
	"github.com/grafana/alerting/receivers/wecom"
)

// BuildReceiverIntegrations creates integrations for each configured notification channel in GrafanaReceiverConfig.
// It returns a slice of Integration objects, one for each notification channel, along with any errors that occurred.
func BuildReceiverIntegrations(
	receiver GrafanaReceiverConfig,
	tmpl *template.Template,
	img images.ImageStore, // Used by some receivers to include as part of the source
	newLogger logging.LoggerFactory,
	newWebhookSender func(n receivers.Metadata) (receivers.WebhookSender, error),
	newEmailSender func(n receivers.Metadata) (receivers.EmailSender, error),
	orgID int64,
	version string,
) ([]*Integration, error) {
	type notificationChannel interface {
		notify.Notifier
		notify.ResolvedSender
	}

	var (
		integrations      []*Integration
		errors            types.MultiError
                // Helper function to create an integration for a notification channel and add it to the integrations slice.
		createIntegration = func(idx int, cfg receivers.Metadata, f func(logger logging.Logger) notificationChannel) {
			logger := newLogger("ngalert.notifier."+cfg.Type, "notifierUID", cfg.UID)
			n := f(logger)
			i := NewIntegration(n, n, cfg.Type, idx)
			integrations = append(integrations, i)
		}
                // Helper function to create an integration for a notification channel that requires a webhook sender.
		createIntegrationWithWebhook = func(idx int, cfg receivers.Metadata, f func(logger logging.Logger, w receivers.WebhookSender) notificationChannel) {
			w, e := newWebhookSender(cfg)
			if e != nil {
				errors.Add(fmt.Errorf("unable to build webhook client for %s notifier %s (UID: %s): %w ", cfg.Type, cfg.Name, cfg.UID, e))
				return
			}
			createIntegration(idx, cfg, func(logger logging.Logger) notificationChannel {
				return f(logger, w)
			})
		}
	)

// Range through each notification channel in the receiver and create an integration for it.
	for i, cfg := range receiver.AlertmanagerConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) notificationChannel {
			return alertmanager.New(cfg.Settings, cfg.Metadata, img, l)
		})
	}
	for i, cfg := range receiver.DingdingConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return dinding.New(cfg.Settings, cfg.Metadata, tmpl, w, l)
		})
	}
	for i, cfg := range receiver.DiscordConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return discord.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l, version)
		})
	}
	for i, cfg := range receiver.EmailConfigs {
		mailCli, e := newEmailSender(cfg.Metadata)
		if e != nil {
			errors.Add(fmt.Errorf("unable to build email client for %s notifier %s (UID: %s): %w ", cfg.Type, cfg.Name, cfg.UID, e))
		}
		createIntegration(i, cfg.Metadata, func(l logging.Logger) notificationChannel {
			return email.New(cfg.Settings, cfg.Metadata, tmpl, mailCli, img, l)
		})
	}
	for i, cfg := range receiver.GooglechatConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return googlechat.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l, version)
		})
	}
	for i, cfg := range receiver.KafkaConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return kafka.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l)
		})
	}
	for i, cfg := range receiver.LineConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return line.New(cfg.Settings, cfg.Metadata, tmpl, w, l)
		})
	}
	for i, cfg := range receiver.OpsgenieConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return opsgenie.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l)
		})
	}
	for i, cfg := range receiver.PagerdutyConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return pagerduty.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l)
		})
	}
	for i, cfg := range receiver.PushoverConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return pushover.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l)
		})
	}
	for i, cfg := range receiver.SensugoConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return sensugo.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l)
		})
	}
	for i, cfg := range receiver.SlackConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return slack.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l, version)
		})
	}
	for i, cfg := range receiver.TeamsConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return teams.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l)
		})
	}
	for i, cfg := range receiver.TelegramConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return telegram.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l)
		})
	}
	for i, cfg := range receiver.ThreemaConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return threema.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l)
		})
	}
	for i, cfg := range receiver.VictoropsConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return victorops.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l, version)
		})
	}
	for i, cfg := range receiver.WebhookConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return webhook.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l, orgID)
		})
	}
	for i, cfg := range receiver.WecomConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return wecom.New(cfg.Settings, cfg.Metadata, tmpl, w, l)
		})
	}
	for i, cfg := range receiver.WebexConfigs {
		createIntegrationWithWebhook(i, cfg.Metadata, func(l logging.Logger, w receivers.WebhookSender) notificationChannel {
			return webex.New(cfg.Settings, cfg.Metadata, tmpl, w, img, l, orgID)
		})
	}

	if errors.Len() > 0 {
		return nil, &errors
	}
	return integrations, nil
}
