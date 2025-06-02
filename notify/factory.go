package notify

import (
	"errors"
	"fmt"
	"slices"

	"github.com/go-kit/log"

	"github.com/grafana/alerting/http"
	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/alertmanager"
	"github.com/grafana/alerting/receivers/dinding"
	"github.com/grafana/alerting/receivers/discord"
	"github.com/grafana/alerting/receivers/email"
	"github.com/grafana/alerting/receivers/googlechat"
	"github.com/grafana/alerting/receivers/jira"
	"github.com/grafana/alerting/receivers/kafka"
	"github.com/grafana/alerting/receivers/line"
	"github.com/grafana/alerting/receivers/mqtt"
	"github.com/grafana/alerting/receivers/oncall"
	"github.com/grafana/alerting/receivers/opsgenie"
	"github.com/grafana/alerting/receivers/pagerduty"
	"github.com/grafana/alerting/receivers/pushover"
	"github.com/grafana/alerting/receivers/sensugo"
	"github.com/grafana/alerting/receivers/slack"
	"github.com/grafana/alerting/receivers/sns"
	"github.com/grafana/alerting/receivers/teams"
	"github.com/grafana/alerting/receivers/telegram"
	"github.com/grafana/alerting/receivers/threema"
	"github.com/grafana/alerting/receivers/victorops"
	"github.com/grafana/alerting/receivers/webex"
	"github.com/grafana/alerting/receivers/webhook"
	"github.com/grafana/alerting/receivers/wecom"
	"github.com/grafana/alerting/templates"
	"github.com/prometheus/alertmanager/notify"
)

type WrapNotifierFunc func(integrationName string, notifier notify.Notifier) notify.Notifier

// BuildReceiverIntegrations creates integrations for each configured notification channel in GrafanaReceiverConfig.
// It returns a slice of Integration objects, one for each notification channel, along with any errors that occurred.
func BuildReceiverIntegrations(
	receiver GrafanaReceiverConfig,
	tmpl *templates.Template,
	img images.Provider,
	logger log.Logger,
	emailSender receivers.EmailSender,
	wrapNotifier WrapNotifierFunc,
	orgID int64,
	version string,
	httpClientOptions ...http.ClientOption,
) ([]*Integration, error) {
	type notificationChannel interface {
		notify.Notifier
		notify.ResolvedSender
	}
	var (
		integrations []*Integration
		errs         error
		ci           = func(idx int, cfg receivers.Metadata, newInt func(cli *http.Client) notificationChannel, opts ...http.ClientOption) {
			client, err := http.NewClient(slices.Concat(httpClientOptions, opts)...)
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("failed to create HTTP client for %q notifier %q (UID: %q): %w", cfg.Type, cfg.Name, cfg.UID, err))
				return
			}
			n := newInt(client)
			notify := wrapNotifier(cfg.Name, n)
			i := NewIntegration(notify, n, cfg.Type, idx, cfg.Name)
			integrations = append(integrations, i)
		}
	)
	// Range through each notification channel in the receiver and create an integration for it.
	for i, cfg := range receiver.AlertmanagerConfigs {
		ci(i, cfg.Metadata, func(_ *http.Client) notificationChannel {
			return alertmanager.New(cfg.Settings, cfg.Metadata, img, logger)
		})
	}
	for i, cfg := range receiver.DingdingConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return dinding.New(cfg.Settings, cfg.Metadata, tmpl, cli, logger)
		})
	}
	for i, cfg := range receiver.DiscordConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return discord.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, version)
		})
	}
	for i, cfg := range receiver.EmailConfigs {
		ci(i, cfg.Metadata, func(_ *http.Client) notificationChannel {
			return email.New(cfg.Settings, cfg.Metadata, tmpl, emailSender, img, logger)
		})
	}
	for i, cfg := range receiver.GooglechatConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return googlechat.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, version)
		})
	}
	for i, cfg := range receiver.JiraConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return jira.New(cfg.Settings, cfg.Metadata, tmpl, http.NewForkedSender(cli), logger)
		})
	}
	for i, cfg := range receiver.KafkaConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return kafka.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.LineConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return line.New(cfg.Settings, cfg.Metadata, tmpl, cli, logger)
		})
	}
	for i, cfg := range receiver.MqttConfigs {
		ci(i, cfg.Metadata, func(_ *http.Client) notificationChannel {
			return mqtt.New(cfg.Settings, cfg.Metadata, tmpl, logger, nil)
		})
	}
	for i, cfg := range receiver.OnCallConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return oncall.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, orgID)
		})
	}
	for i, cfg := range receiver.OpsgenieConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return opsgenie.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.PagerdutyConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return pagerduty.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.PushoverConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return pushover.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.SensugoConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return sensugo.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.SNSConfigs {
		ci(i, cfg.Metadata, func(_ *http.Client) notificationChannel {
			return sns.New(cfg.Settings, cfg.Metadata, tmpl, logger)
		})
	}
	for i, cfg := range receiver.SlackConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return slack.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, version)
		})
	}
	for i, cfg := range receiver.TeamsConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return teams.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.TelegramConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return telegram.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.ThreemaConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return threema.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.VictoropsConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return victorops.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, version)
		})
	}
	for i, cfg := range receiver.WebhookConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return webhook.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, orgID)
		}, http.WithHTTPClientConfig(cfg.Settings.HTTPConfig))
	}
	for i, cfg := range receiver.WecomConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return wecom.New(cfg.Settings, cfg.Metadata, tmpl, cli, logger)
		})
	}
	for i, cfg := range receiver.WebexConfigs {
		ci(i, cfg.Metadata, func(cli *http.Client) notificationChannel {
			return webex.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, orgID)
		})
	}
	return integrations, errs
}
