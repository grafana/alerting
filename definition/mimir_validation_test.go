// copy of https://github.com/grafana/mimir/blob/a5f6bc75e858f2f7ede4b68bd692ed9b4f99193d/pkg/alertmanager/api_test.go

package definition

import (
	"net/url"
	"testing"

	"github.com/prometheus/alertmanager/config"
	commoncfg "github.com/prometheus/common/config"
	"github.com/stretchr/testify/assert"

	httpcfg "github.com/grafana/alerting/http/v0mimir1"
	discord_v0mimir1 "github.com/grafana/alerting/receivers/discord/v0mimir1"
	email_v0mimir1 "github.com/grafana/alerting/receivers/email/v0mimir1"
	opsgenie_v0mimir1 "github.com/grafana/alerting/receivers/opsgenie/v0mimir1"
	pagerduty_v0mimir1 "github.com/grafana/alerting/receivers/pagerduty/v0mimir1"
	pushover_v0mimir1 "github.com/grafana/alerting/receivers/pushover/v0mimir1"
	slack_v0mimir1 "github.com/grafana/alerting/receivers/slack/v0mimir1"
	teams_v0mimir1 "github.com/grafana/alerting/receivers/teams/v0mimir1"
	teams_v0mimir2 "github.com/grafana/alerting/receivers/teams/v0mimir2"
	telegram_v0mimir1 "github.com/grafana/alerting/receivers/telegram/v0mimir1"
	victorops_v0mimir1 "github.com/grafana/alerting/receivers/victorops/v0mimir1"
	webhook_v0mimir1 "github.com/grafana/alerting/receivers/webhook/v0mimir1"
)

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestValidateAlertmanagerConfig(t *testing.T) {
	tests := map[string]struct {
		input    interface{}
		expected error
	}{
		"*HTTPClientConfig": {
			input: &commoncfg.HTTPClientConfig{
				BasicAuth: &commoncfg.BasicAuth{
					PasswordFile: "/secrets",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"HTTPClientConfig": {
			input: commoncfg.HTTPClientConfig{
				BasicAuth: &commoncfg.BasicAuth{
					PasswordFile: "/secrets",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"*TLSConfig": {
			input: &commoncfg.TLSConfig{
				CertFile: "/cert",
			},
			expected: errTLSConfigNotAllowed,
		},
		"TLSConfig": {
			input: commoncfg.TLSConfig{
				CertFile: "/cert",
			},
			expected: errTLSConfigNotAllowed,
		},
		"*GlobalConfig.SMTPAuthPasswordFile": {
			input: &config.GlobalConfig{
				SMTPAuthPasswordFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"GlobalConfig.SMTPAuthPasswordFile": {
			input: config.GlobalConfig{
				SMTPAuthPasswordFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"*DiscordConfig.HTTPConfig": {
			input: &config.DiscordConfig{
				HTTPConfig: &commoncfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"DiscordConfig.HTTPConfig": {
			input: &config.DiscordConfig{
				HTTPConfig: &commoncfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"*DiscordConfig.WebhookURLFile": {
			input: &config.DiscordConfig{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"DiscordConfig.WebhookURLFile": {
			input: config.DiscordConfig{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"*EmailConfig.AuthPasswordFile": {
			input: &config.EmailConfig{
				AuthPasswordFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"EmailConfig.AuthPasswordFile": {
			input: config.EmailConfig{
				AuthPasswordFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"*MSTeams.HTTPConfig": {
			input: &config.MSTeamsConfig{
				HTTPConfig: &commoncfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"MSTeams.HTTPConfig": {
			input: &config.MSTeamsConfig{
				HTTPConfig: &commoncfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"*MSTeams.WebhookURLFile": {
			input: &config.MSTeamsConfig{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"MSTeams.WebhookURLFile": {
			input: config.MSTeamsConfig{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"struct containing *HTTPClientConfig as direct child": {
			input: config.GlobalConfig{
				HTTPConfig: &commoncfg.HTTPClientConfig{
					BasicAuth: &commoncfg.BasicAuth{
						PasswordFile: "/secrets",
					},
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"struct containing *HTTPClientConfig as nested child": {
			input: config.Config{
				Global: &config.GlobalConfig{
					HTTPConfig: &commoncfg.HTTPClientConfig{
						BasicAuth: &commoncfg.BasicAuth{
							PasswordFile: "/secrets",
						},
					},
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"struct containing *HTTPClientConfig as nested child within a slice": {
			input: config.Config{
				Receivers: []config.Receiver{{
					Name: "test",
					WebhookConfigs: []*config.WebhookConfig{{
						HTTPConfig: &commoncfg.HTTPClientConfig{
							BasicAuth: &commoncfg.BasicAuth{
								PasswordFile: "/secrets",
							},
						},
					}}},
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"map containing *HTTPClientConfig": {
			input: map[string]*commoncfg.HTTPClientConfig{
				"test": {
					BasicAuth: &commoncfg.BasicAuth{
						PasswordFile: "/secrets",
					},
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"map containing TLSConfig as nested child": {
			input: map[string][]config.EmailConfig{
				"test": {{
					TLSConfig: commoncfg.TLSConfig{
						CAFile: "/file",
					},
				}},
			},
			expected: errTLSConfigNotAllowed,
		},

		// v0mimir1 types tests
		"v0mimir1 HTTPClientConfig.BasicAuth.PasswordFile": {
			input: httpcfg.HTTPClientConfig{
				BasicAuth: &httpcfg.BasicAuth{
					PasswordFile: "/secrets",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"v0mimir1 HTTPClientConfig.BearerTokenFile": {
			input: httpcfg.HTTPClientConfig{
				BearerTokenFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"v0mimir1 HTTPClientConfig.Authorization.CredentialsFile": {
			input: httpcfg.HTTPClientConfig{
				Authorization: &httpcfg.Authorization{
					CredentialsFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"v0mimir1 HTTPClientConfig.OAuth2.ClientSecretFile": {
			input: httpcfg.HTTPClientConfig{
				OAuth2: &httpcfg.OAuth2{
					ClientID:         "client-id",
					TokenURL:         "http://token-url",
					ClientSecretFile: "/file",
				},
			},
			expected: errOAuth2SecretFileNotAllowed,
		},
		"v0mimir1 HTTPClientConfig.OAuth2.ProxyURL": {
			input: httpcfg.HTTPClientConfig{
				OAuth2: &httpcfg.OAuth2{
					ClientID:     "client-id",
					TokenURL:     "http://token-url",
					ClientSecret: "secret",
					ProxyConfig: httpcfg.ProxyConfig{
						ProxyURL: commoncfg.URL{URL: mustParseURL("http://proxy:8080")},
					},
				},
			},
			expected: errProxyURLNotAllowed,
		},
		"v0mimir1 HTTPClientConfig.OAuth2.ProxyFromEnvironment": {
			input: httpcfg.HTTPClientConfig{
				OAuth2: &httpcfg.OAuth2{
					ClientID:     "client-id",
					TokenURL:     "http://token-url",
					ClientSecret: "secret",
					ProxyConfig: httpcfg.ProxyConfig{
						ProxyFromEnvironment: true,
					},
				},
			},
			expected: errProxyFromEnvironmentURLNotAllowed,
		},
		"v0mimir1 TLSConfig.CAFile": {
			input: httpcfg.TLSConfig{
				CAFile: "/file",
			},
			expected: errTLSConfigNotAllowed,
		},
		"v0mimir1 TLSConfig.CertFile": {
			input: httpcfg.TLSConfig{
				CertFile: "/file",
			},
			expected: errTLSConfigNotAllowed,
		},
		"v0mimir1 TLSConfig.KeyFile": {
			input: httpcfg.TLSConfig{
				KeyFile: "/file",
			},
			expected: errTLSConfigNotAllowed,
		},
		"v0mimir1 TLSConfig.CA": {
			input: httpcfg.TLSConfig{
				CA: "ca-content",
			},
			expected: errTLSConfigNotAllowed,
		},
		"v0mimir1 TLSConfig.Cert": {
			input: httpcfg.TLSConfig{
				Cert: "cert-content",
			},
			expected: errTLSConfigNotAllowed,
		},
		"v0mimir1 TLSConfig.Key": {
			input: httpcfg.TLSConfig{
				Key: "key-content",
			},
			expected: errTLSConfigNotAllowed,
		},
		"v0mimir1 DiscordConfig.WebhookURLFile": {
			input: discord_v0mimir1.Config{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"v0mimir1 EmailConfig.AuthPasswordFile": {
			input: email_v0mimir1.Config{
				AuthPasswordFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"v0mimir1 SlackConfig.APIURLFile": {
			input: slack_v0mimir1.Config{
				APIURLFile: "/file",
			},
			expected: errSlackAPIURLFileNotAllowed,
		},
		"v0mimir1 OpsGenieConfig.APIKeyFile": {
			input: opsgenie_v0mimir1.Config{
				APIKeyFile: "/file",
			},
			expected: errOpsGenieAPIKeyFileFileNotAllowed,
		},
		"v0mimir1 VictorOpsConfig.APIKeyFile": {
			input: victorops_v0mimir1.Config{
				APIKeyFile: "/file",
			},
			expected: errVictorOpsAPIKeyFileNotAllowed,
		},
		"v0mimir1 PagerDutyConfig.ServiceKeyFile": {
			input: pagerduty_v0mimir1.Config{
				ServiceKeyFile: "/file",
			},
			expected: errPagerDutyServiceKeyFileNotAllowed,
		},
		"v0mimir1 PagerDutyConfig.RoutingKeyFile": {
			input: pagerduty_v0mimir1.Config{
				RoutingKeyFile: "/file",
			},
			expected: errPagerDutyRoutingKeyFileNotAllowed,
		},
		"v0mimir1 PushoverConfig.UserKeyFile": {
			input: pushover_v0mimir1.Config{
				UserKeyFile: "/file",
			},
			expected: errPushoverUserKeyFileNotAllowed,
		},
		"v0mimir1 PushoverConfig.TokenFile": {
			input: pushover_v0mimir1.Config{
				TokenFile: "/file",
			},
			expected: errPushoverTokenFileNotAllowed,
		},
		"v0mimir1 TelegramConfig.BotTokenFile": {
			input: telegram_v0mimir1.Config{
				BotTokenFile: "/file",
			},
			expected: errTelegramBotTokenFileNotAllowed,
		},
		"v0mimir1 WebhookConfig.URLFile": {
			input: webhook_v0mimir1.Config{
				URLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"v0mimir1 MSTeamsConfig.WebhookURLFile": {
			input: teams_v0mimir1.Config{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"v0mimir2 MSTeamsV2Config.WebhookURLFile": {
			input: teams_v0mimir2.Config{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"Receiver with v0mimir1 DiscordConfig.WebhookURLFile": {
			input: Receiver{
				Name: "test",
				DiscordConfigs: []*discord_v0mimir1.Config{{
					WebhookURLFile: "/file",
				}},
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"Receiver with v0mimir1 HTTPClientConfig": {
			input: Receiver{
				Name: "test",
				WebhookConfigs: []*webhook_v0mimir1.Config{{
					HTTPConfig: &httpcfg.HTTPClientConfig{
						BasicAuth: &httpcfg.BasicAuth{
							PasswordFile: "/secrets",
						},
					},
				}},
			},
			expected: errPasswordFileNotAllowed,
		},
	}

	for testName, testData := range tests {
		t.Run(testName, func(t *testing.T) {
			err := ValidateAlertmanagerConfig(testData.input)
			assert.ErrorIs(t, err, testData.expected)
		})
	}
}
