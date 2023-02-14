package notify

import (
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"

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

type NotificationChannel interface {
	notify.Notifier
	notify.ResolvedSender
}

// BuildReceiverIntegrations builds notifiers of the receiver and wraps each of them in Integration.
func BuildReceiverIntegrations(
	receiver GrafanaReceiverConfig,
	tmpl *template.Template,
	ns receivers.NotificationSender,
	img images.ImageStore, // Used by some receivers to include as part of the source
	newLogger logging.LoggerFactory,
	orgID int64,
	version string,
) []*Integration {
	var integrations []*Integration

	createIntegration := func(idx int, cfg receivers.Metadata, f func(logger logging.Logger) NotificationChannel) {
		logger := newLogger("ngalert.notifier." + cfg.Type) // TODO add more context here
		n := f(logger)
		i := NewIntegration(n, n, cfg.Type, idx)
		integrations = append(integrations, i)
	}

	for i, cfg := range receiver.AlertmanagerConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return alertmanager.New2(cfg.Settings, cfg.Metadata, img, l)
		})
	}
	for i, cfg := range receiver.DingdingConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return dinding.New2(cfg.Settings, cfg.Metadata, tmpl, ns, l)
		})
	}
	for i, cfg := range receiver.DiscordConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return discord.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l, version)
		})
	}
	for i, cfg := range receiver.EmailConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return email.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l)
		})
	}
	for i, cfg := range receiver.GooglechatConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return googlechat.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l, version)
		})
	}
	for i, cfg := range receiver.KafkaConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return kafka.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l)
		})
	}
	for i, cfg := range receiver.LineConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return line.New2(cfg.Settings, cfg.Metadata, tmpl, ns, l)
		})
	}
	for i, cfg := range receiver.OpsgenieConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return opsgenie.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l)
		})
	}
	for i, cfg := range receiver.PagerdutyConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return pagerduty.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l)
		})
	}
	for i, cfg := range receiver.PushoverConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return pushover.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l)
		})
	}
	for i, cfg := range receiver.SensugoConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return sensugo.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l)
		})
	}
	for i, cfg := range receiver.SlackConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return slack.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l, version)
		})
	}
	for i, cfg := range receiver.TeamsConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return teams.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l)
		})
	}
	for i, cfg := range receiver.TelegramConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return telegram.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l)
		})
	}
	for i, cfg := range receiver.ThreemaConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return threema.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l)
		})
	}
	for i, cfg := range receiver.VictoropsConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return victorops.New2(cfg.Settings, cfg.Metadata, tmpl, ns, img, l, version)
		})
	}
	for i, cfg := range receiver.WebhookConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return webhook.New2(cfg.Settings, orgID, cfg.Metadata, tmpl, ns, img, l)
		})
	}
	for i, cfg := range receiver.WecomConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return wecom.New2(cfg.Settings, cfg.Metadata, tmpl, ns, l)
		})
	}
	for i, cfg := range receiver.WebexConfigs {
		createIntegration(i, cfg.Metadata, func(l logging.Logger) NotificationChannel {
			return webex.New2(cfg.Settings, orgID, cfg.Metadata, tmpl, ns, img, l)
		})
	}

	return integrations
}
