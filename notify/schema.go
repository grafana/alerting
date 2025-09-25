package notify

import (
	"github.com/grafana/alerting/receivers/alertmanager"
	"github.com/grafana/alerting/receivers/dingding"
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
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/sensugo"
	"github.com/grafana/alerting/receivers/slack"
	"github.com/grafana/alerting/receivers/sns"
	"github.com/grafana/alerting/receivers/teams"
	"github.com/grafana/alerting/receivers/telegram"
	"github.com/grafana/alerting/receivers/threema"
	"github.com/grafana/alerting/receivers/victorops"
	"github.com/grafana/alerting/receivers/webex"
	"github.com/grafana/alerting/receivers/webhook"
	"github.com/grafana/alerting/receivers/wechat"
	"github.com/grafana/alerting/receivers/wecom"
)

// GetSchemaForAllIntegrations returns the current schema for all integrations.
func GetSchemaForAllIntegrations() map[schema.IntegrationType]schema.IntegrationTypeSchema {
	return map[schema.IntegrationType]schema.IntegrationTypeSchema{
		alertmanager.Type: alertmanager.Schema(),
		dingding.Type:     dingding.Schema(),
		discord.Type:      discord.Schema(),
		email.Type:        email.Schema(),
		googlechat.Type:   googlechat.Schema(),
		jira.Type:         jira.Schema(),
		kafka.Type:        kafka.Schema(),
		line.Type:         line.Schema(),
		mqtt.Type:         mqtt.Schema(),
		oncall.Type:       oncall.Schema(),
		opsgenie.Type:     opsgenie.Schema(),
		pagerduty.Type:    pagerduty.Schema(),
		pushover.Type:     pushover.Schema(),
		sensugo.Type:      sensugo.Schema(),
		slack.Type:        slack.Schema(),
		sns.Type:          sns.Schema(),
		teams.Type:        teams.Schema(),
		telegram.Type:     telegram.Schema(),
		threema.Type:      threema.Schema(),
		victorops.Type:    victorops.Schema(),
		webex.Type:        webex.Schema(),
		webhook.Type:      webhook.Schema(),
		wecom.Type:        wecom.Schema(),
		wechat.Type:       wechat.Schema(),
	}
}
