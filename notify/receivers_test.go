package notify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
	t.Run("AllKnownConfigsForTesting contains all notifier types", func(t *testing.T) {
		// Sanity check to ensure this fails when not all notifier types are present in the configuration.
		// If this doesn't pass, other tests that rely on this function will not be reliable.
		_, missing := allReceivers(&GrafanaReceiverConfig{})
		require.Greaterf(t, missing, 0, "all notifier types should be missing, allReceivers may no longer be reliable, missing: %d", missing)

		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NoError(t, err)
		_, missing = allReceivers(&parsed)
		require.Equalf(t, 0, missing, "all notifier types should be present, missing: %d", missing)
	})
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
		// We decode all the secureSettings from base64 and then build then receivers. The test is to ensure that
		// BuildReceiverConfiguration can handle the already decoded secrets correctly.
		for notifierType, cfg := range AllKnownConfigsForTesting {
			notifierRaw := cfg.GetRawNotifierConfig(notifierType)
			if len(notifierRaw.SecureSettings) == 0 {
				continue
			}
			for key := range notifierRaw.SecureSettings {
				decoded, err := base64.StdEncoding.DecodeString(notifierRaw.SecureSettings[key])
				require.NoError(t, err)
				notifierRaw.SecureSettings[key] = string(decoded)
			}
			recCfg.Integrations = append(recCfg.Integrations, notifierRaw)
		}

		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, NoopDecode, NoopDecrypt)
		require.NoError(t, err)
		require.Equal(t, recCfg.Name, parsed.Name)
		for _, notifier := range recCfg.GrafanaIntegrations.Integrations {
			if notifier.Type == "prometheus-alertmanager" {
				require.Equal(t, notifier.SecureSettings["basicAuthPassword"], parsed.AlertmanagerConfigs[0].Settings.Password)
			}
		}

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

		expectedNotifiers := make(map[string]struct{})
		for _, notifier := range recCfg.GrafanaIntegrations.Integrations {
			expectedNotifiers[notifier.Type] = struct{}{}
		}

		// Ensure that one of every notifier is present in the parsed configuration.
		all, _ := allReceivers(&parsed)
		require.Len(t, all, len(AllKnownConfigsForTesting), "mismatch in number of notifiers, expected %d, got %d", len(AllKnownConfigsForTesting), len(all))
		for _, recv := range all {
			if _, ok := expectedNotifiers[recv.Metadata.Type]; ok {
				delete(expectedNotifiers, recv.Metadata.Type)
			} else {
				t.Errorf("unexpected notifier type: %s", recv.Metadata.Type)
			}
		}
		require.Empty(t, expectedNotifiers, "not all expected notifiers were found in the parsed configuration")

		t.Run("should populate metadata", func(t *testing.T) {
			for idx, cfg := range all {
				meta := cfg.Metadata
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

		expectedNotifiers := make(map[string]struct{})
		for _, notifier := range recCfg.GrafanaIntegrations.Integrations {
			expectedNotifiers[notifier.Type] = struct{}{}
		}

		// Ensure that one of every notifier is present in the parsed configuration.
		all, _ := allReceivers(&parsed)
		require.Len(t, all, len(AllKnownConfigsForTesting), "mismatch in number of notifiers, expected %d, got %d", len(AllKnownConfigsForTesting), len(all))
		for _, recv := range all {
			if _, ok := expectedNotifiers[recv.Metadata.Type]; ok {
				delete(expectedNotifiers, recv.Metadata.Type)
			} else {
				t.Errorf("unexpected notifier type: %s", recv.Metadata.Type)
			}
		}
		require.Empty(t, expectedNotifiers, "not all expected notifiers were found in the parsed configuration")
	})
}

func allReceivers(r *GrafanaReceiverConfig) ([]NotifierConfig[any], int) {
	var recvs []NotifierConfig[any]
	data, _ := json.Marshal(r)
	var asMap map[string][]NotifierConfig[any]
	_ = json.Unmarshal(data, &asMap)

	notifierConfigPrefix := reflect.TypeOf((*NotifierConfig[any])(nil)).Elem().Name()
	notifierConfigPrefix = notifierConfigPrefix[:strings.Index(notifierConfigPrefix, "[")+1]
	isNotifierConfigField := func(name string) bool {
		field, ok := reflect.TypeOf(GrafanaReceiverConfig{}).FieldByName(name)
		if !ok || field.Type.Kind() != reflect.Slice {
			return false
		}

		if !strings.HasPrefix(field.Type.Elem().Elem().Name(), notifierConfigPrefix) {
			return false
		}
		return true
	}

	missing := 0
	for k, configs := range asMap {
		if !isNotifierConfigField(k) {
			// Skip fields that are not of type []*NotifierConfig.
			continue
		}
		if len(configs) == 0 {
			missing++
			continue
		}
		recvs = append(recvs, configs...)
	}
	return recvs, missing
}
