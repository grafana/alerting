package notifytest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/grafana/alerting/http"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/receivers/alertmanager"
	alertmanagerv1 "github.com/grafana/alerting/receivers/alertmanager/v1"
	"github.com/grafana/alerting/receivers/dingding"
	dingdingv1 "github.com/grafana/alerting/receivers/dingding/v1"
	"github.com/grafana/alerting/receivers/discord"
	discordv0mimir1 "github.com/grafana/alerting/receivers/discord/v0mimir1"
	discordv1 "github.com/grafana/alerting/receivers/discord/v1"
	"github.com/grafana/alerting/receivers/email"
	emailv0mimir1 "github.com/grafana/alerting/receivers/email/v0mimir1"
	emailv1 "github.com/grafana/alerting/receivers/email/v1"
	"github.com/grafana/alerting/receivers/googlechat"
	googlechatv1 "github.com/grafana/alerting/receivers/googlechat/v1"
	"github.com/grafana/alerting/receivers/jira"
	jirav0mimir1 "github.com/grafana/alerting/receivers/jira/v0mimir1"
	jirav1 "github.com/grafana/alerting/receivers/jira/v1"
	"github.com/grafana/alerting/receivers/kafka"
	kafkav1 "github.com/grafana/alerting/receivers/kafka/v1"
	"github.com/grafana/alerting/receivers/line"
	linev1 "github.com/grafana/alerting/receivers/line/v1"
	"github.com/grafana/alerting/receivers/mqtt"
	mqttv1 "github.com/grafana/alerting/receivers/mqtt/v1"
	"github.com/grafana/alerting/receivers/oncall"
	oncallv1 "github.com/grafana/alerting/receivers/oncall/v1"
	"github.com/grafana/alerting/receivers/opsgenie"
	opsgeniev0mimir1 "github.com/grafana/alerting/receivers/opsgenie/v0mimir1"
	opsgeniev1 "github.com/grafana/alerting/receivers/opsgenie/v1"
	"github.com/grafana/alerting/receivers/pagerduty"
	pagerdutyv0mimir1 "github.com/grafana/alerting/receivers/pagerduty/v0mimir1"
	pagerdutyv1 "github.com/grafana/alerting/receivers/pagerduty/v1"
	"github.com/grafana/alerting/receivers/pushover"
	pushoverv0mimir1 "github.com/grafana/alerting/receivers/pushover/v0mimir1"
	pushoverv1 "github.com/grafana/alerting/receivers/pushover/v1"
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/sensugo"
	sensugov1 "github.com/grafana/alerting/receivers/sensugo/v1"
	"github.com/grafana/alerting/receivers/slack"
	slackv0mimir1 "github.com/grafana/alerting/receivers/slack/v0mimir1"
	slackv1 "github.com/grafana/alerting/receivers/slack/v1"
	"github.com/grafana/alerting/receivers/sns"
	snsv0mimir1 "github.com/grafana/alerting/receivers/sns/v0mimir1"
	snsv1 "github.com/grafana/alerting/receivers/sns/v1"
	"github.com/grafana/alerting/receivers/teams"
	teamsv0mimir1 "github.com/grafana/alerting/receivers/teams/v0mimir1"
	teamsv0mimir2 "github.com/grafana/alerting/receivers/teams/v0mimir2"
	teamsv1 "github.com/grafana/alerting/receivers/teams/v1"
	"github.com/grafana/alerting/receivers/telegram"
	telegramv0mimir1 "github.com/grafana/alerting/receivers/telegram/v0mimir1"
	telegramv1 "github.com/grafana/alerting/receivers/telegram/v1"
	"github.com/grafana/alerting/receivers/threema"
	threemav1 "github.com/grafana/alerting/receivers/threema/v1"
	"github.com/grafana/alerting/receivers/victorops"
	victoropsv0mimir1 "github.com/grafana/alerting/receivers/victorops/v0mimir1"
	victoropsv1 "github.com/grafana/alerting/receivers/victorops/v1"
	"github.com/grafana/alerting/receivers/webex"
	webexv0mimir1 "github.com/grafana/alerting/receivers/webex/v0mimir1"
	webexv1 "github.com/grafana/alerting/receivers/webex/v1"
	"github.com/grafana/alerting/receivers/webhook"
	webhookv0mimir1 "github.com/grafana/alerting/receivers/webhook/v0mimir1"
	webhookv1 "github.com/grafana/alerting/receivers/webhook/v1"
	"github.com/grafana/alerting/receivers/wechat"
	wechatv0mimir1 "github.com/grafana/alerting/receivers/wechat/v0mimir1"
	"github.com/grafana/alerting/receivers/wecom"
	wecomv1 "github.com/grafana/alerting/receivers/wecom/v1"
)

var AllKnownV1ConfigsForTesting = map[schema.IntegrationType]NotifierConfigTest{
	alertmanager.Type: {
		NotifierType:                alertmanager.Type,
		Version:                     schema.V1,
		Config:                      alertmanagerv1.FullValidConfigForTesting,
		Secrets:                     alertmanagerv1.FullValidSecretsForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	dingding.Type: {
		NotifierType: dingding.Type,
		Version:      schema.V1,
		Config:       dingdingv1.FullValidConfigForTesting,
		Secrets:      dingdingv1.FullValidSecretsForTesting,
	},
	discord.Type: {
		NotifierType: discord.Type,
		Version:      schema.V1,
		Config:       discordv1.FullValidConfigForTesting,
	},
	email.Type: {
		NotifierType:                email.Type,
		Version:                     schema.V1,
		Config:                      emailv1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	googlechat.Type: {
		NotifierType: googlechat.Type,
		Version:      schema.V1,
		Config:       googlechatv1.FullValidConfigForTesting,
		Secrets:      googlechatv1.FullValidSecretsForTesting,
	},
	jira.Type: {
		NotifierType: jira.Type,
		Version:      schema.V1,
		Config:       jirav1.FullValidConfigForTesting,
		Secrets:      jirav1.FullValidSecretsForTesting,
	},
	kafka.Type: {
		NotifierType: kafka.Type,
		Version:      schema.V1,
		Config:       kafkav1.FullValidConfigForTesting,
		Secrets:      kafkav1.FullValidSecretsForTesting,
	},
	line.Type: {
		NotifierType: line.Type,
		Version:      schema.V1,
		Config:       linev1.FullValidConfigForTesting,
		Secrets:      linev1.FullValidSecretsForTesting,
	},
	mqtt.Type: {
		NotifierType:                mqtt.Type,
		Version:                     schema.V1,
		Config:                      mqttv1.FullValidConfigForTesting,
		Secrets:                     mqttv1.FullValidSecretsForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	oncall.Type: {
		NotifierType: oncall.Type,
		Version:      schema.V1,
		Config:       oncallv1.FullValidConfigForTesting,
		Secrets:      oncallv1.FullValidSecretsForTesting,
	},
	opsgenie.Type: {
		NotifierType: opsgenie.Type,
		Version:      schema.V1,
		Config:       opsgeniev1.FullValidConfigForTesting,
		Secrets:      opsgeniev1.FullValidSecretsForTesting,
	},
	pagerduty.Type: {
		NotifierType: pagerduty.Type,
		Version:      schema.V1,
		Config:       pagerdutyv1.FullValidConfigForTesting,
		Secrets:      pagerdutyv1.FullValidSecretsForTesting,
	},
	pushover.Type: {
		NotifierType: pushover.Type,
		Version:      schema.V1,
		Config:       pushoverv1.FullValidConfigForTesting,
		Secrets:      pushoverv1.FullValidSecretsForTesting,
	},
	sensugo.Type: {
		NotifierType: sensugo.Type,
		Version:      schema.V1,
		Config:       sensugov1.FullValidConfigForTesting,
		Secrets:      sensugov1.FullValidSecretsForTesting,
	},
	slack.Type: {
		NotifierType:                slack.Type,
		Version:                     schema.V1,
		Config:                      slackv1.FullValidConfigForTesting,
		Secrets:                     slackv1.FullValidSecretsForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	sns.Type: {
		NotifierType:                sns.Type,
		Version:                     schema.V1,
		Config:                      snsv1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	teams.Type: {
		NotifierType: teams.Type,
		Version:      schema.V1,
		Config:       teamsv1.FullValidConfigForTesting,
	},
	telegram.Type: {
		NotifierType: telegram.Type,
		Version:      schema.V1,
		Config:       telegramv1.FullValidConfigForTesting,
		Secrets:      telegramv1.FullValidSecretsForTesting,
	},
	threema.Type: {
		NotifierType: threema.Type,
		Version:      schema.V1,
		Config:       threemav1.FullValidConfigForTesting,
		Secrets:      threemav1.FullValidSecretsForTesting,
	},
	victorops.Type: {
		NotifierType: victorops.Type,
		Version:      schema.V1,
		Config:       victoropsv1.FullValidConfigForTesting,
		Secrets:      victoropsv1.FullValidSecretsForTesting,
	},
	webhook.Type: {
		NotifierType: webhook.Type,
		Version:      schema.V1,
		Config:       webhookv1.FullValidConfigForTesting,
		Secrets:      webhookv1.FullValidSecretsForTesting,
	},
	wecom.Type: {
		NotifierType: wecom.Type,
		Version:      schema.V1,
		Config:       wecomv1.FullValidConfigForTesting,
		Secrets:      wecomv1.FullValidSecretsForTesting,
	},
	webex.Type: {
		NotifierType: webex.Type,
		Version:      schema.V1,
		Config:       webexv1.FullValidConfigForTesting,
		Secrets:      webexv1.FullValidSecretsForTesting,
	},
}

var FullValidHTTPConfigForTesting = fmt.Sprintf(`{
	"http_config": {
		"oauth2": {
			"client_id": "test-client-id",
			"client_secret": "test-client-secret",
			"token_url": "https://localhost/auth/token",
			"scopes": ["scope1", "scope2"],
			"endpoint_params": {
				"param1": "value1",
				"param2": "value2"
			},
			"tls_config": {
				"insecureSkipVerify": false,
				"clientCertificate": %[1]q,
				"clientKey": %[2]q,
				"caCertificate": %[3]q
			},
			"proxy_config": {
				"proxy_url": "http://localproxy:8080",
				"no_proxy": "localhost",
				"proxy_from_environment": false,
				"proxy_connect_header": {
					"X-Proxy-Header": "proxy-value"
				}
			}
		}
    }
}`, http.TestCertPem, http.TestKeyPem, http.TestCACert)

var FullValidHTTPConfigSecretsForTesting = fmt.Sprintf(`{
	"http_config.oauth2.client_secret": "test-override-oauth2-secret",
	"http_config.oauth2.tls_config.clientCertificate": %[1]q,
	"http_config.oauth2.tls_config.clientKey": %[2]q,
	"http_config.oauth2.tls_config.caCertificate": %[3]q
}`, http.TestCertPem, http.TestKeyPem, http.TestCACert)

type NotifierConfigTest struct {
	NotifierType                schema.IntegrationType
	Version                     schema.Version
	Config                      string
	Secrets                     string
	CommonHTTPConfigUnsupported bool
}

// IntegrationVersionKey is a composite key combining integration type and version,
// used to uniquely identify a specific versioned integration configuration.
type IntegrationVersionKey struct {
	Type    schema.IntegrationType
	Version schema.Version
}

// AllKnownConfigsForTesting contains valid test configurations for all known integrations
// across all versions (V1, V0mimir1, V0mimir2), keyed by (IntegrationType, Version).
var AllKnownConfigsForTesting = map[IntegrationVersionKey]NotifierConfigTest{
	// V1
	{alertmanager.Type, schema.V1}: {
		NotifierType:                alertmanager.Type,
		Version:                     schema.V1,
		Config:                      alertmanagerv1.FullValidConfigForTesting,
		Secrets:                     alertmanagerv1.FullValidSecretsForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{dingding.Type, schema.V1}: {
		NotifierType: dingding.Type,
		Version:      schema.V1,
		Config:       dingdingv1.FullValidConfigForTesting,
		Secrets:      dingdingv1.FullValidSecretsForTesting,
	},
	{discord.Type, schema.V1}: {
		NotifierType: discord.Type,
		Version:      schema.V1,
		Config:       discordv1.FullValidConfigForTesting,
	},
	{email.Type, schema.V1}: {
		NotifierType:                email.Type,
		Version:                     schema.V1,
		Config:                      emailv1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{googlechat.Type, schema.V1}: {
		NotifierType: googlechat.Type,
		Version:      schema.V1,
		Config:       googlechatv1.FullValidConfigForTesting,
		Secrets:      googlechatv1.FullValidSecretsForTesting,
	},
	{jira.Type, schema.V1}: {
		NotifierType: jira.Type,
		Version:      schema.V1,
		Config:       jirav1.FullValidConfigForTesting,
		Secrets:      jirav1.FullValidSecretsForTesting,
	},
	{kafka.Type, schema.V1}: {
		NotifierType: kafka.Type,
		Version:      schema.V1,
		Config:       kafkav1.FullValidConfigForTesting,
		Secrets:      kafkav1.FullValidSecretsForTesting,
	},
	{line.Type, schema.V1}: {
		NotifierType: line.Type,
		Version:      schema.V1,
		Config:       linev1.FullValidConfigForTesting,
		Secrets:      linev1.FullValidSecretsForTesting,
	},
	{mqtt.Type, schema.V1}: {
		NotifierType:                mqtt.Type,
		Version:                     schema.V1,
		Config:                      mqttv1.FullValidConfigForTesting,
		Secrets:                     mqttv1.FullValidSecretsForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{oncall.Type, schema.V1}: {
		NotifierType: oncall.Type,
		Version:      schema.V1,
		Config:       oncallv1.FullValidConfigForTesting,
		Secrets:      oncallv1.FullValidSecretsForTesting,
	},
	{opsgenie.Type, schema.V1}: {
		NotifierType: opsgenie.Type,
		Version:      schema.V1,
		Config:       opsgeniev1.FullValidConfigForTesting,
		Secrets:      opsgeniev1.FullValidSecretsForTesting,
	},
	{pagerduty.Type, schema.V1}: {
		NotifierType: pagerduty.Type,
		Version:      schema.V1,
		Config:       pagerdutyv1.FullValidConfigForTesting,
		Secrets:      pagerdutyv1.FullValidSecretsForTesting,
	},
	{pushover.Type, schema.V1}: {
		NotifierType: pushover.Type,
		Version:      schema.V1,
		Config:       pushoverv1.FullValidConfigForTesting,
		Secrets:      pushoverv1.FullValidSecretsForTesting,
	},
	{sensugo.Type, schema.V1}: {
		NotifierType: sensugo.Type,
		Version:      schema.V1,
		Config:       sensugov1.FullValidConfigForTesting,
		Secrets:      sensugov1.FullValidSecretsForTesting,
	},
	{slack.Type, schema.V1}: {
		NotifierType:                slack.Type,
		Version:                     schema.V1,
		Config:                      slackv1.FullValidConfigForTesting,
		Secrets:                     slackv1.FullValidSecretsForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{sns.Type, schema.V1}: {
		NotifierType:                sns.Type,
		Version:                     schema.V1,
		Config:                      snsv1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{teams.Type, schema.V1}: {
		NotifierType: teams.Type,
		Version:      schema.V1,
		Config:       teamsv1.FullValidConfigForTesting,
	},
	{telegram.Type, schema.V1}: {
		NotifierType: telegram.Type,
		Version:      schema.V1,
		Config:       telegramv1.FullValidConfigForTesting,
		Secrets:      telegramv1.FullValidSecretsForTesting,
	},
	{threema.Type, schema.V1}: {
		NotifierType: threema.Type,
		Version:      schema.V1,
		Config:       threemav1.FullValidConfigForTesting,
		Secrets:      threemav1.FullValidSecretsForTesting,
	},
	{victorops.Type, schema.V1}: {
		NotifierType: victorops.Type,
		Version:      schema.V1,
		Config:       victoropsv1.FullValidConfigForTesting,
		Secrets:      victoropsv1.FullValidSecretsForTesting,
	},
	{webhook.Type, schema.V1}: {
		NotifierType: webhook.Type,
		Version:      schema.V1,
		Config:       webhookv1.FullValidConfigForTesting,
		Secrets:      webhookv1.FullValidSecretsForTesting,
	},
	{wecom.Type, schema.V1}: {
		NotifierType: wecom.Type,
		Version:      schema.V1,
		Config:       wecomv1.FullValidConfigForTesting,
		Secrets:      wecomv1.FullValidSecretsForTesting,
	},
	{webex.Type, schema.V1}: {
		NotifierType: webex.Type,
		Version:      schema.V1,
		Config:       webexv1.FullValidConfigForTesting,
		Secrets:      webexv1.FullValidSecretsForTesting,
	},
	// V0mimir1
	{discord.Type, schema.V0mimir1}: {
		NotifierType:                discord.Type,
		Version:                     schema.V0mimir1,
		Config:                      discordv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{email.Type, schema.V0mimir1}: {
		NotifierType:                email.Type,
		Version:                     schema.V0mimir1,
		Config:                      emailv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{jira.Type, schema.V0mimir1}: {
		NotifierType:                jira.Type,
		Version:                     schema.V0mimir1,
		Config:                      jirav0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{opsgenie.Type, schema.V0mimir1}: {
		NotifierType:                opsgenie.Type,
		Version:                     schema.V0mimir1,
		Config:                      opsgeniev0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{pagerduty.Type, schema.V0mimir1}: {
		NotifierType:                pagerduty.Type,
		Version:                     schema.V0mimir1,
		Config:                      pagerdutyv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{pushover.Type, schema.V0mimir1}: {
		NotifierType:                pushover.Type,
		Version:                     schema.V0mimir1,
		Config:                      pushoverv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{slack.Type, schema.V0mimir1}: {
		NotifierType:                slack.Type,
		Version:                     schema.V0mimir1,
		Config:                      slackv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{sns.Type, schema.V0mimir1}: {
		NotifierType:                sns.Type,
		Version:                     schema.V0mimir1,
		Config:                      snsv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{teams.Type, schema.V0mimir1}: {
		NotifierType:                teams.Type,
		Version:                     schema.V0mimir1,
		Config:                      teamsv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{telegram.Type, schema.V0mimir1}: {
		NotifierType:                telegram.Type,
		Version:                     schema.V0mimir1,
		Config:                      telegramv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{victorops.Type, schema.V0mimir1}: {
		NotifierType:                victorops.Type,
		Version:                     schema.V0mimir1,
		Config:                      victoropsv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{webex.Type, schema.V0mimir1}: {
		NotifierType:                webex.Type,
		Version:                     schema.V0mimir1,
		Config:                      webexv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{webhook.Type, schema.V0mimir1}: {
		NotifierType:                webhook.Type,
		Version:                     schema.V0mimir1,
		Config:                      webhookv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	{wechat.Type, schema.V0mimir1}: {
		NotifierType:                wechat.Type,
		Version:                     schema.V0mimir1,
		Config:                      wechatv0mimir1.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
	// V0mimir2
	{teams.Type, schema.V0mimir2}: {
		NotifierType:                teams.Type,
		Version:                     schema.V0mimir2,
		Config:                      teamsv0mimir2.FullValidConfigForTesting,
		CommonHTTPConfigUnsupported: true,
	},
}

func (n NotifierConfigTest) GetRawNotifierConfig(name string) *models.IntegrationConfig {
	if name == "" {
		name = string(n.NotifierType)
	}
	secrets := make(map[string]string)
	if n.Secrets != "" {
		err := json.Unmarshal([]byte(n.Secrets), &secrets)
		if err != nil {
			panic(err)
		}
		for key, value := range secrets {
			secrets[key] = base64.StdEncoding.EncodeToString([]byte(value))
		}
	}

	config := []byte(n.Config)
	if !n.CommonHTTPConfigUnsupported {
		var err error
		config, err = mergeSettings([]byte(n.Config), []byte(FullValidHTTPConfigForTesting))
		if err != nil {
			panic(err)
		}
	}

	return &models.IntegrationConfig{
		UID:                   fmt.Sprintf("%s-uid", name),
		Name:                  name,
		Type:                  n.NotifierType,
		Version:               n.Version,
		DisableResolveMessage: true,
		Settings:              config,
		SecureSettings:        secrets,
	}
}
