package notify

import (
	"strings"

	"github.com/prometheus/alertmanager/notify"

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

var receiverFactories = map[string]func(receivers.FactoryConfig) (NotificationChannel, error){
	"prometheus-alertmanager": wrap(alertmanager.New),
	"dingding":                wrap(dinding.New),
	"discord":                 wrap(discord.New),
	"email":                   wrap(email.New),
	"googlechat":              wrap(googlechat.New),
	"kafka":                   wrap(kafka.New),
	"line":                    wrap(line.New),
	"opsgenie":                wrap(opsgenie.New),
	"pagerduty":               wrap(pagerduty.New),
	"pushover":                wrap(pushover.New),
	"sensugo":                 wrap(sensugo.New),
	"slack":                   wrap(slack.New),
	"teams":                   wrap(teams.New),
	"telegram":                wrap(telegram.New),
	"threema":                 wrap(threema.New),
	"victorops":               wrap(victorops.New),
	"webhook":                 wrap(webhook.New),
	"wecom":                   wrap(wecom.New),
	"webex":                   wrap(webex.New),
}

type NotificationChannel interface {
	notify.Notifier
	notify.ResolvedSender
}

func Factory(receiverType string) (func(receivers.FactoryConfig) (NotificationChannel, error), bool) {
	receiverType = strings.ToLower(receiverType)
	factory, exists := receiverFactories[receiverType]
	return factory, exists
}

// wrap wraps the notifier's factory errors with receivers.ReceiverInitError
func wrap[T NotificationChannel](f func(fc receivers.FactoryConfig) (T, error)) func(receivers.FactoryConfig) (NotificationChannel, error) {
	return func(fc receivers.FactoryConfig) (NotificationChannel, error) {
		ch, err := f(fc)
		if err != nil {
			return nil, ReceiverInitError{
				Err: err,
				// TODO it will be removed in the next PR
				Cfg: &GrafanaReceiver{
					UID:                   fc.Config.UID,
					Name:                  fc.Config.Name,
					Type:                  fc.Config.Type,
					DisableResolveMessage: fc.Config.DisableResolveMessage,
					Settings:              fc.Config.Settings,
					SecureSettings:        nil,
				},
			}
		}
		return ch, nil
	}
}
