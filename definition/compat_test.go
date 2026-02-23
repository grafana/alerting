package definition

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/pkg/labels"
)

var validConfig = []byte(`{"route":{"receiver":"grafana-default-email","routes":[{"receiver":"grafana-default-email","object_matchers":[["a","=","b"]],"mute_time_intervals":["test1"]}]},"mute_time_intervals":[{"name":"test1","time_intervals":[{"times":[{"start_time":"00:00","end_time":"12:00"}]}]}],"templates":null,"receivers":[{"name":"grafana-default-email","grafana_managed_receiver_configs":[{"uid":"uxwfZvtnz","name":"email receiver","type":"email","disableResolveMessage":false,"settings":{"addresses":"<example@email.com>"},"secureFields":{}}]}]}`)

func TestLoadCompat(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		expErr string
	}{
		{
			name:   "no configuration",
			input:  []byte(``),
			expErr: "empty input",
		},
		{
			name:   "no routes",
			input:  []byte(`{}`),
			expErr: "no routes provided",
		},
		{
			name:   "duplicated receivers",
			input:  []byte(testConfigDuplicatedReceivers),
			expErr: "notification config name \"test\" is not unique",
		},
		{
			name:  "no global config",
			input: []byte(testConfigWithoutGlobal),
		},
		{
			name:   "no slack api url",
			input:  []byte(fmt.Sprintf(missingValuesTemplate, "slack_configs")),
			expErr: "no global Slack API URL set",
		},
		{
			name:   "no Opsgenie api key",
			input:  []byte(fmt.Sprintf(missingValuesTemplate, "opsgenie_configs")),
			expErr: "no global OpsGenie API Key set",
		},
		{
			name:   "no WeChat api secret",
			input:  []byte(fmt.Sprintf(missingValuesTemplate, "wechat_configs")),
			expErr: "no global Wechat ApiSecret set",
		},
		{
			name:   "no VictorOps api key",
			input:  []byte(fmt.Sprintf(missingValuesTemplate, "victorops_configs")),
			expErr: "no global VictorOps API Key set",
		},
		{
			name:   "no Discord url",
			input:  []byte(fmt.Sprintf(missingValuesTemplate, "discord_configs")),
			expErr: "one of webhook_url or webhook_url_file must be configured",
		},
		{
			name:   "no MSTeams url",
			input:  []byte(fmt.Sprintf(missingValuesTemplate, "msteams_configs")),
			expErr: "one of webhook_url or webhook_url_file must be configured",
		},
		{
			name:   "no smarthost",
			input:  []byte(fmt.Sprintf(missingValuesTemplate, "email_configs")),
			expErr: "no global SMTP smarthost set",
		},
		{
			name:  "with global config",
			input: []byte(testConfigWithGlobal),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := LoadCompat(test.input)
			if test.expErr != "" {
				require.Error(t, err)
				require.Equal(t, test.expErr, err.Error())
				return
			}

			require.NoError(t, err)

			// It should add the default global config.
			require.NotNil(t, c.Global)
			globalConfig := c.Global

			// All configs should have the default http config set except for Webex.
			require.Equal(t, c.Receivers[0].DiscordConfigs[0].HTTPConfig, globalConfig.HTTPConfig)
			require.Equal(t, c.Receivers[0].MSTeamsConfigs[0].HTTPConfig, globalConfig.HTTPConfig)
			require.Equal(t, c.Receivers[0].OpsGenieConfigs[0].HTTPConfig, globalConfig.HTTPConfig)
			require.Equal(t, c.Receivers[0].PagerdutyConfigs[0].HTTPConfig, globalConfig.HTTPConfig)
			require.Equal(t, c.Receivers[0].PushoverConfigs[0].HTTPConfig, globalConfig.HTTPConfig)
			require.Equal(t, c.Receivers[0].SNSConfigs[0].HTTPConfig, globalConfig.HTTPConfig)
			require.Equal(t, c.Receivers[0].SlackConfigs[0].HTTPConfig, globalConfig.HTTPConfig)
			require.Equal(t, c.Receivers[0].TelegramConfigs[0].HTTPConfig, globalConfig.HTTPConfig)
			require.Equal(t, c.Receivers[0].VictorOpsConfigs[0].HTTPConfig, globalConfig.HTTPConfig)
			require.Equal(t, c.Receivers[0].WebhookConfigs[0].HTTPConfig, globalConfig.HTTPConfig)
			require.Equal(t, c.Receivers[0].WechatConfigs[0].HTTPConfig, globalConfig.HTTPConfig)

			if len(c.Receivers[0].EmailConfigs) > 0 {
				require.Equal(t, c.Receivers[0].EmailConfigs[0].Smarthost, globalConfig.SMTPSmarthost)
				require.Equal(t, c.Receivers[0].EmailConfigs[0].From, globalConfig.SMTPFrom)
				require.Equal(t, c.Receivers[0].EmailConfigs[0].AuthUsername, globalConfig.SMTPAuthUsername)
				require.Equal(t, c.Receivers[0].EmailConfigs[0].AuthPassword, globalConfig.SMTPAuthPassword)
				require.Equal(t, c.Receivers[0].EmailConfigs[0].AuthSecret, globalConfig.SMTPAuthSecret)
				require.Equal(t, c.Receivers[0].EmailConfigs[0].AuthIdentity, globalConfig.SMTPAuthIdentity)
				require.Equal(t, *c.Receivers[0].EmailConfigs[0].RequireTLS, globalConfig.SMTPRequireTLS)
			}
		})
	}

}

func TestAsAMRoute(t *testing.T) {
	// Ensure that AsAMRoute and AsGrafanaRoute are inverses of each other.
	cfg, err := LoadCompat([]byte(testConfigWithComplexRoutes))
	require.NoError(t, err)
	originalRoute := cfg.Route
	// For easier comparison move ObjectMatchers to Matchers.
	mergeMatchers(originalRoute)

	amRoute := originalRoute.AsAMRoute()
	grafanaRoute := AsGrafanaRoute(amRoute)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreUnexported(Route{}, labels.Matcher{}),
		cmpopts.EquateEmpty(),
	}
	if !cmp.Equal(grafanaRoute, originalRoute, cmpOpts...) {
		t.Errorf("Unexpected Diff: %v", cmp.Diff(grafanaRoute, originalRoute, cmpOpts...))
	}
}

func mergeMatchers(route *Route) {
	route.Matchers = append(route.Matchers, route.ObjectMatchers...)
	route.ObjectMatchers = nil
	for _, r := range route.Routes {
		mergeMatchers(r)
	}
}

const testConfigWithoutGlobal = `
route:
  receiver: test
  routes:
    - receiver: test
receivers:
  - name: test
    discord_configs:
      - webhook_url: http://test.com
    msteams_configs:
      - webhook_url: http://test.com
    opsgenie_configs:
      - api_key: test
    pagerduty_configs:
      - routing_key: test
    pushover_configs:
      - user_key: test
        token: test
    slack_configs:
      - api_url: http://test.com
    sns_configs:
      - topic_arn: test
    telegram_configs:
      - bot_token: test
        chat_id: 1
    victorops_configs:
      - api_key: test
        routing_key: test
    webhook_configs:
      - url: http://test.com
    wechat_configs:
      - api_key: test
        api_secret: test
        corp_id: test
    webex_configs:
      - api_url: http://test.com
        room_id: test
        http_config:
          bearer_token: test
`

const testConfigWithGlobal = `
global:
  smtp_smarthost: smtp.example.org:587
  smtp_from: testfrom@test.com
  resolve_timeout: 5m
  http_config:
    follow_redirects: false
    enable_http2: false
  smtp_hello: test
  smtp_require_tls: false
  pagerduty_url: https://pagerdutytest.com
  slack_api_url: https://slacktest.com
  opsgenie_api_url: https://opsgenietest.com
  opsgenie_api_key: test
  wechat_api_url: https://wechattest.com
  wechat_api_secret: test
  wechat_api_corp_id: test_id
  victorops_api_url: https://victoropstest.com
  victorops_api_key: test
  telegram_api_url: https://telegramtest.com
  webex_api_url: https://webextest.com
route:
  receiver: test
  routes:
    - receiver: test
receivers:
  - name: test
    email_configs:
      - to: test
    discord_configs:
      - webhook_url: http://test.com
    msteams_configs:
      - webhook_url: http://test.com
    opsgenie_configs:
      - send_resolved: true
    pagerduty_configs:
      - routing_key: test
    pushover_configs:
      - user_key: test
        token: test
    slack_configs:
      - channel: test
    sns_configs:
      - topic_arn: test
    telegram_configs:
      - bot_token: test
        chat_id: 1
    victorops_configs:
      - routing_key: test
    webhook_configs:
      - url: http://test.com
    wechat_configs:
      - api_key: test
    webex_configs:
      - room_id: test
        http_config:
          bearer_token: test
`

const testConfigDuplicatedReceivers = `
route:
  receiver: test
  routes:
    - receiver: test
receivers:
  - name: test
  - name: test
`

const missingValuesTemplate = `
global:
  resolve_timeout: 5m
  http_config:
    follow_redirects: false
    enable_http2: false
route:
  receiver: test
  routes:
    - receiver: test
receivers:
  - name: test
    %s:
      - send_resolved: false
        routing_key: test
        to: test
`

const testConfigWithComplexRoutes = `
mute_time_intervals:
  - name: test1
    time_intervals:
      - times:
          - start_time: 00:00
            end_time: 12:00
time_intervals:
  - name: weekends
    time_intervals:
    - weekdays:
      - saturday
      - sunday
  - name: weekdays
    time_intervals:
    - weekdays:
      - monday
      - tuesday
      - wednesday
      - thursday
      - friday
route:
  receiver: recv
  group_by:
    - test
    - test2
  group_wait: 1m
  group_interval: 1m
  repeat_interval: 1m
  routes:
    - receiver: recv2
      object_matchers:
        - - team
          - =
          - teamC
      group_by:
        - teste
        - test2f
      group_wait: 0s
      group_interval: 1m
      repeat_interval: 1m
      mute_time_intervals:
        - test1
      active_time_intervals:
        - weekdays
      routes:
        - receiver: recv
          group_by:
            - testc
            - test2d
          group_interval: 10m
          repeat_interval: 1h
          mute_time_intervals:
            - weekends
          active_time_intervals:
            - weekdays
          routes:
            - receiver: recv2
              group_by:
                - testa
                - test2b
              group_wait: 30s
              group_interval: 1m
              repeat_interval: 1m
              active_time_intervals:
                - weekdays
                - test1
receivers:
  - name: recv
    webhook_configs:
      - url: http://localhost:8080/alert
  - name: recv2
    webhook_configs::
      - url: http://localhost:8080/alert
`
