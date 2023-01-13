package notifier

import (
	"strings"

	"github.com/grafana/alerting/alerting/notifier/channels"
	"github.com/grafana/alerting/alerting/notifier/config"
)

var receiverFactories = map[string]func(config.FactoryConfig) (channels.NotificationChannel, error){
	"prometheus-alertmanager": channels.AlertmanagerFactory,
	"dingding":                channels.DingDingFactory,
	"discord":                 channels.DiscordFactory,
	"email":                   channels.EmailFactory,
	"googlechat":              channels.GoogleChatFactory,
	"kafka":                   channels.KafkaFactory,
	"line":                    channels.LineFactory,
	"opsgenie":                channels.OpsgenieFactory,
	"pagerduty":               channels.PagerdutyFactory,
	"pushover":                channels.PushoverFactory,
	"sensugo":                 channels.SensuGoFactory,
	"slack":                   channels.SlackFactory,
	"teams":                   channels.TeamsFactory,
	"telegram":                channels.TelegramFactory,
	"threema":                 channels.ThreemaFactory,
	"victorops":               channels.VictorOpsFactory,
	"webhook":                 channels.WebHookFactory,
	"wecom":                   channels.WeComFactory,
	"webex":                   channels.WebexFactory,
}

func Factory(receiverType string) (func(config.FactoryConfig) (channels.NotificationChannel, error), bool) {
	receiverType = strings.ToLower(receiverType)
	factory, exists := receiverFactories[receiverType]
	return factory, exists
}
