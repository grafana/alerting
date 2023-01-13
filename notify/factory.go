package notify

import (
	"strings"

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

var receiverFactories = map[string]func(receivers.FactoryConfig) (receivers.NotificationChannel, error){
	"prometheus-alertmanager": alertmanager.AlertmanagerFactory,
	"dingding":                dinding.DingDingFactory,
	"discord":                 discord.DiscordFactory,
	"email":                   email.EmailFactory,
	"googlechat":              googlechat.GoogleChatFactory,
	"kafka":                   kafka.KafkaFactory,
	"line":                    line.LineFactory,
	"opsgenie":                opsgenie.OpsgenieFactory,
	"pagerduty":               pagerduty.PagerdutyFactory,
	"pushover":                pushover.PushoverFactory,
	"sensugo":                 sensugo.SensuGoFactory,
	"slack":                   slack.SlackFactory,
	"teams":                   teams.TeamsFactory,
	"telegram":                telegram.TelegramFactory,
	"threema":                 threema.ThreemaFactory,
	"victorops":               victorops.VictorOpsFactory,
	"webhook":                 webhook.WebHookFactory,
	"wecom":                   wecom.WeComFactory,
	"webex":                   webex.WebexFactory,
}

func Factory(receiverType string) (func(receivers.FactoryConfig) (receivers.NotificationChannel, error), bool) {
	receiverType = strings.ToLower(receiverType)
	factory, exists := receiverFactories[receiverType]
	return factory, exists
}
