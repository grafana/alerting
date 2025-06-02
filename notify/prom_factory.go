package notify

import (
	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/notify"
	promMsteams "github.com/prometheus/alertmanager/notify/msteams"
	promWechat "github.com/prometheus/alertmanager/notify/wechat"
	"github.com/prometheus/alertmanager/types"
	commoncfg "github.com/prometheus/common/config"

	promDiscord "github.com/prometheus/alertmanager/notify/discord"
	promEmail "github.com/prometheus/alertmanager/notify/email"
	promOpsgenie "github.com/prometheus/alertmanager/notify/opsgenie"
	promPagerduty "github.com/prometheus/alertmanager/notify/pagerduty"
	promPushover "github.com/prometheus/alertmanager/notify/pushover"
	promSlack "github.com/prometheus/alertmanager/notify/slack"
	promSns "github.com/prometheus/alertmanager/notify/sns"
	promTelegram "github.com/prometheus/alertmanager/notify/telegram"
	promVictorops "github.com/prometheus/alertmanager/notify/victorops"
	promWebex "github.com/prometheus/alertmanager/notify/webex"
	promWebhook "github.com/prometheus/alertmanager/notify/webhook"
	"github.com/prometheus/alertmanager/template"

	"github.com/grafana/alerting/notify/nfstatus"
)

// BuildPromReceiverIntegrations builds a list of integration notifiers off of a receiver config.
// Taken from https://github.com/prometheus/alertmanager/blob/94d875f1227b29abece661db1a68c001122d1da5/cmd/alertmanager/main.go#L112-L159.
func BuildPromReceiverIntegrations(nc config.Receiver, tmpl *template.Template, httpOps []commoncfg.HTTPClientOption, logger log.Logger, wrapper func(string, notify.Notifier) notify.Notifier) ([]*nfstatus.Integration, error) {
	var (
		errs         types.MultiError
		integrations []*nfstatus.Integration
		add          = func(name string, i int, rs notify.ResolvedSender, f func(l log.Logger) (notify.Notifier, error)) {
			n, err := f(log.With(logger, "integration", name))
			if err != nil {
				errs.Add(err)
				return
			}
			if wrapper != nil {
				n = wrapper(name, n)
			}
			integrations = append(integrations, nfstatus.NewIntegration(n, rs, name, i, nc.Name))
		}
	)

	for i, c := range nc.WebhookConfigs {
		add("webhook", i, c, func(l log.Logger) (notify.Notifier, error) { return promWebhook.New(c, tmpl, l, httpOps...) })
	}
	for i, c := range nc.EmailConfigs {
		add("email", i, c, func(l log.Logger) (notify.Notifier, error) { return promEmail.New(c, tmpl, l), nil })
	}
	for i, c := range nc.PagerdutyConfigs {
		add("pagerduty", i, c, func(l log.Logger) (notify.Notifier, error) { return promPagerduty.New(c, tmpl, l, httpOps...) })
	}
	for i, c := range nc.OpsGenieConfigs {
		add("opsgenie", i, c, func(l log.Logger) (notify.Notifier, error) { return promOpsgenie.New(c, tmpl, l, httpOps...) })
	}
	for i, c := range nc.WechatConfigs {
		add("wechat", i, c, func(l log.Logger) (notify.Notifier, error) { return promWechat.New(c, tmpl, l, httpOps...) })
	}
	for i, c := range nc.SlackConfigs {
		add("slack", i, c, func(l log.Logger) (notify.Notifier, error) { return promSlack.New(c, tmpl, l, httpOps...) })
	}
	for i, c := range nc.VictorOpsConfigs {
		add("victorops", i, c, func(l log.Logger) (notify.Notifier, error) { return promVictorops.New(c, tmpl, l, httpOps...) })
	}
	for i, c := range nc.PushoverConfigs {
		add("pushover", i, c, func(l log.Logger) (notify.Notifier, error) { return promPushover.New(c, tmpl, l, httpOps...) })
	}
	for i, c := range nc.SNSConfigs {
		add("sns", i, c, func(l log.Logger) (notify.Notifier, error) { return promSns.New(c, tmpl, l, httpOps...) })
	}
	for i, c := range nc.TelegramConfigs {
		add("telegram", i, c, func(l log.Logger) (notify.Notifier, error) { return promTelegram.New(c, tmpl, l, httpOps...) })
	}
	for i, c := range nc.DiscordConfigs {
		add("discord", i, c, func(l log.Logger) (notify.Notifier, error) { return promDiscord.New(c, tmpl, l, httpOps...) })
	}
	for i, c := range nc.WebexConfigs {
		add("webex", i, c, func(l log.Logger) (notify.Notifier, error) { return promWebex.New(c, tmpl, l, httpOps...) })
	}
	for i, c := range nc.MSTeamsConfigs {
		add("msteams", i, c, func(l log.Logger) (notify.Notifier, error) { return promMsteams.New(c, tmpl, l, httpOps...) })
	}
	// If we add support for more integrations, we need to add them to validation as well. See validation.allowedIntegrationNames field.
	if errs.Len() > 0 {
		return nil, &errs
	}
	return integrations, nil
}
