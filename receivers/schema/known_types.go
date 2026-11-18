package schema

const (
	AlertManagerType IntegrationType = "prometheus-alertmanager"
	DingDingType     IntegrationType = "dingding"
	DiscordType      IntegrationType = "discord"
	EmailType        IntegrationType = "email"
	GoogleChatType   IntegrationType = "googlechat"
	JiraType         IntegrationType = "jira"
	KafkaType        IntegrationType = "kafka"
	// LineType is retained as a compatibility tombstone so persisted LINE
	// contact points can be ignored safely after the integration's removal.
	LineType      IntegrationType = "LINE"
	MQTTType      IntegrationType = "mqtt"
	OnCallType    IntegrationType = "oncall"
	OpsGenieType  IntegrationType = "opsgenie"
	PagerDutyType IntegrationType = "pagerduty"
	PushoverType  IntegrationType = "pushover"
	SensuGoType   IntegrationType = "sensugo"
	SlackType     IntegrationType = "slack"
	SNSType       IntegrationType = "sns"
	TeamsType     IntegrationType = "teams"
	TelegramType  IntegrationType = "telegram"
	ThreemaType   IntegrationType = "threema"
	VictorOpsType IntegrationType = "victorops"
	WebexType     IntegrationType = "webex"
	WebhookType   IntegrationType = "webhook"
	WeChatType    IntegrationType = "wechat"
	WeComType     IntegrationType = "wecom"
)
