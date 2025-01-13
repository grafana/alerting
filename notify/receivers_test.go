package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
)

func TestReceiverTimeoutError_Error(t *testing.T) {
	e := IntegrationTimeoutError{
		Integration: &GrafanaIntegrationConfig{
			Name: "test",
			UID:  "uid",
		},
		Err: errors.New("context deadline exceeded"),
	}
	require.Equal(t, "the receiver timed out: context deadline exceeded", e.Error())
}

type timeoutError struct{}

func (e timeoutError) Error() string {
	return "the request timed out"
}

func (e timeoutError) Timeout() bool {
	return true
}

func TestProcessNotifierError(t *testing.T) {
	t.Run("assert IntegrationTimeoutError is returned for context deadline exceeded", func(t *testing.T) {
		r := &GrafanaIntegrationConfig{
			Name: "test",
			UID:  "uid",
		}
		require.Equal(t, IntegrationTimeoutError{
			Integration: r,
			Err:         context.DeadlineExceeded,
		}, ProcessIntegrationError(r, context.DeadlineExceeded))
	})

	t.Run("assert IntegrationTimeoutError is returned for *url.Error timeout", func(t *testing.T) {
		r := &GrafanaIntegrationConfig{
			Name: "test",
			UID:  "uid",
		}
		urlError := &url.Error{
			Op:  "Get",
			URL: "https://grafana.net",
			Err: timeoutError{},
		}
		require.Equal(t, IntegrationTimeoutError{
			Integration: r,
			Err:         urlError,
		}, ProcessIntegrationError(r, urlError))
	})

	t.Run("assert unknown error is returned unmodified", func(t *testing.T) {
		r := &GrafanaIntegrationConfig{
			Name: "test",
			UID:  "uid",
		}
		err := errors.New("this is an error")
		require.Equal(t, err, ProcessIntegrationError(r, err))
	})
}

func TestBuildReceiverConfiguration(t *testing.T) {
	decrypt := GetDecryptedValueFnForTesting
	t.Run("should decode secrets from base64", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		counter := 0
		decryptCount := func(ctx context.Context, sjd map[string][]byte, key string, fallback string) string {
			counter++
			return decrypt(ctx, sjd, key, fallback)
		}
		_, _ = BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decryptCount)
		require.Greater(t, counter, 0)
	})
	t.Run("should fail if at least one config is invalid", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		bad := &GrafanaIntegrationConfig{
			UID:      "invalid-test",
			Name:     "invalid-test",
			Type:     "slack",
			Settings: json.RawMessage(`{ "test" : "test" }`),
		}
		recCfg.Integrations = append(recCfg.Integrations, bad)

		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NotNil(t, err)
		require.Equal(t, GrafanaReceiverConfig{}, parsed)
		require.ErrorAs(t, err, &IntegrationValidationError{})
		typedError := err.(IntegrationValidationError)
		require.NotNil(t, typedError.Integration)
		require.Equal(t, bad, typedError.Integration)
		require.ErrorContains(t, err, fmt.Sprintf(`failed to validate integration "%s" (UID %s) of type "%s"`, bad.Name, bad.UID, bad.Type))
	})
	t.Run("should accept empty config", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NoError(t, err)
		require.Equal(t, recCfg.Name, parsed.Name)
	})
	t.Run("should support non-base64-encoded secrets", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		invalidBase64 := "test"
		for notifierType, cfg := range AllKnownConfigsForTesting {
			notifierRaw := cfg.GetRawNotifierConfig(notifierType)
			if len(notifierRaw.SecureSettings) == 0 {
				continue
			}
			for key := range notifierRaw.SecureSettings {
				notifierRaw.SecureSettings[key] = invalidBase64
			}
			recCfg.Integrations = append(recCfg.Integrations, notifierRaw)
		}

		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, NoopDecode, NoopDecrypt)
		require.NoError(t, err)
		require.Equal(t, recCfg.Name, parsed.Name)
		require.Equal(t, invalidBase64, parsed.AlertmanagerConfigs[0].Settings.Password)
	})
	t.Run("should fail if notifier type is unknown", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		bad := &GrafanaIntegrationConfig{
			UID:      "test",
			Name:     "test",
			Type:     fmt.Sprintf("invalid-%d", rand.Uint32()),
			Settings: json.RawMessage(`{ "test" : "test" }`),
		}
		recCfg.Integrations = append(recCfg.Integrations, bad)

		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NotNil(t, err)
		require.Equal(t, GrafanaReceiverConfig{}, parsed)
		require.ErrorAs(t, err, &IntegrationValidationError{})
		typedError := err.(IntegrationValidationError)
		require.NotNil(t, typedError.Integration)
		require.Equal(t, bad, typedError.Integration)
		require.ErrorContains(t, err, fmt.Sprintf("notifier %s is not supported", bad.Type))
	})
	t.Run("should recognize all known types", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NoError(t, err)
		require.Equal(t, recCfg.Name, parsed.Name)
		require.Len(t, parsed.AlertmanagerConfigs, 1)
		require.Len(t, parsed.DingdingConfigs, 1)
		require.Len(t, parsed.DiscordConfigs, 1)
		require.Len(t, parsed.EmailConfigs, 1)
		require.Len(t, parsed.GooglechatConfigs, 1)
		require.Len(t, parsed.KafkaConfigs, 1)
		require.Len(t, parsed.LineConfigs, 1)
		require.Len(t, parsed.OpsgenieConfigs, 1)
		require.Len(t, parsed.PagerdutyConfigs, 1)
		require.Len(t, parsed.PushoverConfigs, 1)
		require.Len(t, parsed.SensugoConfigs, 1)
		require.Len(t, parsed.SlackConfigs, 1)
		require.Len(t, parsed.SNSConfigs, 1)
		require.Len(t, parsed.TeamsConfigs, 1)
		require.Len(t, parsed.TelegramConfigs, 1)
		require.Len(t, parsed.ThreemaConfigs, 1)
		require.Len(t, parsed.VictoropsConfigs, 1)
		require.Len(t, parsed.WebhookConfigs, 1)
		require.Len(t, parsed.WecomConfigs, 1)
		require.Len(t, parsed.WebexConfigs, 1)

		t.Run("should populate metadata", func(t *testing.T) {
			var all []receivers.Metadata
			all = append(all, getMetadata(parsed.AlertmanagerConfigs)...)
			all = append(all, getMetadata(parsed.DingdingConfigs)...)
			all = append(all, getMetadata(parsed.DiscordConfigs)...)
			all = append(all, getMetadata(parsed.EmailConfigs)...)
			all = append(all, getMetadata(parsed.GooglechatConfigs)...)
			all = append(all, getMetadata(parsed.KafkaConfigs)...)
			all = append(all, getMetadata(parsed.LineConfigs)...)
			all = append(all, getMetadata(parsed.OpsgenieConfigs)...)
			all = append(all, getMetadata(parsed.PagerdutyConfigs)...)
			all = append(all, getMetadata(parsed.PushoverConfigs)...)
			all = append(all, getMetadata(parsed.SensugoConfigs)...)
			all = append(all, getMetadata(parsed.SlackConfigs)...)
			all = append(all, getMetadata(parsed.SNSConfigs)...)
			all = append(all, getMetadata(parsed.TeamsConfigs)...)
			all = append(all, getMetadata(parsed.TelegramConfigs)...)
			all = append(all, getMetadata(parsed.ThreemaConfigs)...)
			all = append(all, getMetadata(parsed.VictoropsConfigs)...)
			all = append(all, getMetadata(parsed.WebhookConfigs)...)
			all = append(all, getMetadata(parsed.WecomConfigs)...)
			all = append(all, getMetadata(parsed.WebexConfigs)...)

			for idx, meta := range all {
				require.NotEmptyf(t, meta.Type, "%s notifier (idx: %d) '%s' uid: '%s'.", meta.Type, idx, meta.Name, meta.UID)
				require.NotEmptyf(t, meta.UID, "%s notifier (idx: %d) '%s' uid: '%s'.", meta.Type, idx, meta.Name, meta.UID)
				require.NotEmptyf(t, meta.Name, "%s notifier (idx: %d) '%s' uid: '%s'.", meta.Type, idx, meta.Name, meta.UID)
				var notifierRaw *GrafanaIntegrationConfig
				for _, receiver := range recCfg.Integrations {
					if receiver.Type == meta.Type && receiver.UID == meta.UID && receiver.Name == meta.Name {
						notifierRaw = receiver
						break
					}
				}
				require.NotNilf(t, notifierRaw, "cannot find raw settings for %s notifier '%s' uid: '%s'.", meta.Type, meta.Name, meta.UID)
				require.Equalf(t, notifierRaw.DisableResolveMessage, meta.DisableResolveMessage, "%s notifier '%s' uid: '%s'.", meta.Type, meta.Name, meta.UID)
			}
		})
	})
	t.Run("should recognize type in any case", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			notifierRaw := cfg.GetRawNotifierConfig(notifierType)
			notifierRaw.Type = strings.ToUpper(notifierRaw.Type)
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NoError(t, err)
		require.Len(t, parsed.AlertmanagerConfigs, 1)
		require.Len(t, parsed.DingdingConfigs, 1)
		require.Len(t, parsed.DiscordConfigs, 1)
		require.Len(t, parsed.EmailConfigs, 1)
		require.Len(t, parsed.GooglechatConfigs, 1)
		require.Len(t, parsed.KafkaConfigs, 1)
		require.Len(t, parsed.LineConfigs, 1)
		require.Len(t, parsed.OpsgenieConfigs, 1)
		require.Len(t, parsed.PagerdutyConfigs, 1)
		require.Len(t, parsed.PushoverConfigs, 1)
		require.Len(t, parsed.SensugoConfigs, 1)
		require.Len(t, parsed.SlackConfigs, 1)
		require.Len(t, parsed.SNSConfigs, 1)
		require.Len(t, parsed.TeamsConfigs, 1)
		require.Len(t, parsed.TelegramConfigs, 1)
		require.Len(t, parsed.ThreemaConfigs, 1)
		require.Len(t, parsed.VictoropsConfigs, 1)
		require.Len(t, parsed.WebhookConfigs, 1)
		require.Len(t, parsed.WecomConfigs, 1)
		require.Len(t, parsed.WebexConfigs, 1)

	})
}

func getMetadata[T any](notifiers []*NotifierConfig[T]) []receivers.Metadata {
	result := make([]receivers.Metadata, 0, len(notifiers))
	for _, notifier := range notifiers {
		result = append(result, notifier.Metadata)
	}
	return result
}
