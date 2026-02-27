package notifytest

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"

	"github.com/grafana/alerting/definition"
	"github.com/grafana/alerting/http/v0mimir1/v0mimir1test"
	discordV0 "github.com/grafana/alerting/receivers/discord/v0mimir1"
	emailV0 "github.com/grafana/alerting/receivers/email/v0mimir1"
	jiraV0 "github.com/grafana/alerting/receivers/jira/v0mimir1"
	opsgenieV0 "github.com/grafana/alerting/receivers/opsgenie/v0mimir1"
	pagerdutyV0 "github.com/grafana/alerting/receivers/pagerduty/v0mimir1"
	pushoverV0 "github.com/grafana/alerting/receivers/pushover/v0mimir1"
	slackV0 "github.com/grafana/alerting/receivers/slack/v0mimir1"
	snsV0 "github.com/grafana/alerting/receivers/sns/v0mimir1"
	msteamsV01 "github.com/grafana/alerting/receivers/teams/v0mimir1"
	msteamsV02 "github.com/grafana/alerting/receivers/teams/v0mimir2"
	telegramV0 "github.com/grafana/alerting/receivers/telegram/v0mimir1"
	victoropsV0 "github.com/grafana/alerting/receivers/victorops/v0mimir1"
	webexV0 "github.com/grafana/alerting/receivers/webex/v0mimir1"
	webhookV0 "github.com/grafana/alerting/receivers/webhook/v0mimir1"
	wechatV0 "github.com/grafana/alerting/receivers/wechat/v0mimir1"
)

// GetMimirIntegration creates a new instance of the given integration type with selected http config options.
// It panics if the configuration process encounters an issue.
func GetMimirIntegration[T any](opts ...v0mimir1test.MimirIntegrationHTTPConfigOption) (T, error) {
	var config T
	cfg, err := GetRawConfigForMimirIntegration(reflect.TypeOf(config), opts...)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal([]byte(cfg), &config)
	if err != nil {
		return config, fmt.Errorf("failed to unmarshal config %T: %v", config, err)
	}
	return config, nil
}

// GetMimirIntegrationForType creates a new instance of the given integration type with selected http config options.
// It panics if the configuration process encounters an issue.
func GetMimirIntegrationForType(iType reflect.Type, opts ...v0mimir1test.MimirIntegrationHTTPConfigOption) (any, error) {
	cfg, err := GetRawConfigForMimirIntegration(iType, opts...)
	if err != nil {
		return nil, err
	}
	elemPtr := reflect.New(iType).Interface()
	err = json.Unmarshal([]byte(cfg), elemPtr)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config %T: %v", iType, err)
	}
	return elemPtr, nil
}

// GetMimirReceiverWithIntegrations creates a Receiver with selected integrations configured from given types and options.
// It returns a Receiver for testing purposes or an error if the configuration process encounters an issue.
func GetMimirReceiverWithIntegrations(iTypes []reflect.Type, opts ...v0mimir1test.MimirIntegrationHTTPConfigOption) (definition.Receiver, error) {
	receiver := definition.Receiver{Name: "receiver"}
	receiverVal := reflect.ValueOf(&receiver).Elem()
	receiverType := receiverVal.Type()
	for i := 0; i < receiverType.NumField(); i++ {
		integrationField := receiverType.Field(i)
		if integrationField.Type.Kind() != reflect.Slice {
			continue
		}
		sliceType := integrationField.Type
		elemType := sliceType.Elem()

		sliceVal := reflect.MakeSlice(sliceType, 0, 1)

		// Create a new instance of the element type
		elemPtr := reflect.New(elemType).Interface()
		underlyingType := elemType
		if underlyingType.Kind() == reflect.Ptr {
			underlyingType = underlyingType.Elem()
		}
		if !slices.Contains(iTypes, underlyingType) {
			continue
		}
		rawConfig, err := GetRawConfigForMimirIntegration(underlyingType, opts...)
		if err != nil {
			return definition.Receiver{}, fmt.Errorf("failed to get config for type [%s]: %v", underlyingType.String(), err)
		}
		if err := json.Unmarshal([]byte(rawConfig), elemPtr); err != nil {
			return definition.Receiver{}, fmt.Errorf("failed to parse config for type %s: %v", elemType.String(), err)
		}
		sliceVal = reflect.Append(sliceVal, reflect.ValueOf(elemPtr).Elem())
		receiverVal.FieldByName(integrationField.Name).Set(sliceVal)
	}
	return receiver, nil
}

// GetMimirReceiverWithAllIntegrations creates a Receiver with all integrations configured from given types and options.
// It returns a Receiver for testing purposes or an error if the configuration process encounters an issue.
func GetMimirReceiverWithAllIntegrations(opts ...v0mimir1test.MimirIntegrationHTTPConfigOption) (definition.Receiver, error) {
	return GetMimirReceiverWithIntegrations(slices.Collect(maps.Keys(AllValidMimirConfigs)), opts...)
}

func GetRawConfigForMimirIntegration(iType reflect.Type, opts ...v0mimir1test.MimirIntegrationHTTPConfigOption) (string, error) {
	cfg, ok := AllValidMimirConfigs[iType]
	if !ok {
		return "", fmt.Errorf("invalid config type [%s", iType.String())
	}
	if _, ok := iType.FieldByName("HTTPConfig"); !ok { // ignore integrations without HTTPConfig
		return cfg, nil
	}
	if len(opts) == 0 {
		opts = []v0mimir1test.MimirIntegrationHTTPConfigOption{v0mimir1test.WithDefault}
	}
	for _, opt := range opts {
		c, ok := v0mimir1test.ValidMimirHTTPConfigs[opt]
		if !ok {
			return "", fmt.Errorf("invalid option [%s]", opt)
		}
		bytes, err := mergeSettings([]byte(cfg), []byte(c))
		if err != nil {
			return "", fmt.Errorf("failed to merge config for type [%s] with options [%s]: %v", iType.String(), opt, err)
		}
		cfg = string(bytes)
	}
	return cfg, nil
}

// FullValidMimirReceiver builds a Receiver with all integration types populated
// using GetFullValidConfig from each integration package.
func FullValidMimirReceiver() definition.Receiver {
	discord := discordV0.GetFullValidConfig()
	email := emailV0.GetFullValidConfig()
	jira := jiraV0.GetFullValidConfig()
	opsgenie := opsgenieV0.GetFullValidConfig()
	pagerduty := pagerdutyV0.GetFullValidConfig()
	pushover := pushoverV0.GetFullValidConfig()
	slack := slackV0.GetFullValidConfig()
	sns := snsV0.GetFullValidConfig()
	teamsV1 := msteamsV01.GetFullValidConfig()
	teamsV2 := msteamsV02.GetFullValidConfig()
	telegram := telegramV0.GetFullValidConfig()
	victorops := victoropsV0.GetFullValidConfig()
	webex := webexV0.GetFullValidConfig()
	webhook := webhookV0.GetFullValidConfig()
	wechat := wechatV0.GetFullValidConfig()

	return definition.Receiver{
		Name:             "test-receiver",
		DiscordConfigs:   []*discordV0.Config{&discord},
		EmailConfigs:     []*emailV0.Config{&email},
		JiraConfigs:      []*jiraV0.Config{&jira},
		OpsGenieConfigs:  []*opsgenieV0.Config{&opsgenie},
		PagerdutyConfigs: []*pagerdutyV0.Config{&pagerduty},
		PushoverConfigs:  []*pushoverV0.Config{&pushover},
		SlackConfigs:     []*slackV0.Config{&slack},
		SNSConfigs:       []*snsV0.Config{&sns},
		MSTeamsConfigs:   []*msteamsV01.Config{&teamsV1},
		MSTeamsV2Configs: []*msteamsV02.Config{&teamsV2},
		TelegramConfigs:  []*telegramV0.Config{&telegram},
		VictorOpsConfigs: []*victoropsV0.Config{&victorops},
		WebexConfigs:     []*webexV0.Config{&webex},
		WebhookConfigs:   []*webhookV0.Config{&webhook},
		WechatConfigs:    []*wechatV0.Config{&wechat},
	}
}

var AllValidMimirConfigs = map[reflect.Type]string{
	reflect.TypeOf(discordV0.Config{}):   discordV0.FullValidConfigForTesting,
	reflect.TypeOf(emailV0.Config{}):     emailV0.FullValidConfigForTesting,
	reflect.TypeOf(pagerdutyV0.Config{}): pagerdutyV0.FullValidConfigForTesting,
	reflect.TypeOf(slackV0.Config{}):     slackV0.FullValidConfigForTesting,
	reflect.TypeOf(webhookV0.Config{}):   webhookV0.FullValidConfigForTesting,
	reflect.TypeOf(opsgenieV0.Config{}):  opsgenieV0.FullValidConfigForTesting,
	reflect.TypeOf(wechatV0.Config{}):    wechatV0.FullValidConfigForTesting,
	reflect.TypeOf(pushoverV0.Config{}):  pushoverV0.FullValidConfigForTesting,
	reflect.TypeOf(victoropsV0.Config{}): victoropsV0.FullValidConfigForTesting,
	// all sigv4 fields of SNSConfig are different in yaml
	reflect.TypeOf(snsV0.Config{}): snsV0.FullValidConfigForTesting,
	// token and chat fields of TelegramConfig are different in yaml
	reflect.TypeOf(telegramV0.Config{}): telegramV0.FullValidConfigForTesting,
	reflect.TypeOf(webexV0.Config{}):    webexV0.FullValidConfigForTesting,
	reflect.TypeOf(msteamsV01.Config{}): msteamsV01.FullValidConfigForTesting,
	reflect.TypeOf(msteamsV02.Config{}): msteamsV02.FullValidConfigForTesting,
	reflect.TypeOf(jiraV0.Config{}):     jiraV0.FullValidConfigForTesting,
}
