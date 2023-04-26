package notify

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestBuildReceiverIntegrations(t *testing.T) {
	var orgID = rand.Int63()
	var version = fmt.Sprintf("Grafana v%d", rand.Uint32())
	imageProvider := &images.FakeProvider{}
	tmpl := templates.ForTests(t)

	webhookFactory := func(n receivers.Metadata) (receivers.WebhookSender, error) {
		return receivers.MockNotificationService(), nil
	}
	emailFactory := func(n receivers.Metadata) (receivers.EmailSender, error) {
		return receivers.MockNotificationService(), nil
	}
	loggerFactory := func(_ string, _ ...interface{}) logging.Logger {
		return &logging.FakeLogger{}
	}

	getFullConfig := func(t *testing.T) (GrafanaReceiverConfig, int) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range allKnownConfigs {
			recCfg.Integrations = append(recCfg.Integrations, cfg.getRawNotifierConfig(notifierType))
		}
		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, GetDecryptedValueFnForTesting)
		require.NoError(t, err)
		return parsed, len(recCfg.Integrations)
	}

	t.Run("should build all supported notifiers", func(t *testing.T) {
		fullCfg, qty := getFullConfig(t)

		loggerNames := make(map[string]struct{}, qty)
		logger := func(name string, _ ...interface{}) logging.Logger {
			loggerNames[name] = struct{}{}
			return &logging.FakeLogger{}
		}

		webhooks := make(map[receivers.Metadata]struct{}, qty)
		wh := func(n receivers.Metadata) (receivers.WebhookSender, error) {
			webhooks[n] = struct{}{}
			return webhookFactory(n)
		}

		emails := make(map[receivers.Metadata]struct{}, qty)
		em := func(n receivers.Metadata) (receivers.EmailSender, error) {
			emails[n] = struct{}{}
			return emailFactory(n)
		}

		integrations, err := BuildReceiverIntegrations(fullCfg, tmpl, imageProvider, logger, wh, em, orgID, version)

		require.NoError(t, err)
		require.Len(t, integrations, qty)

		t.Run("should call logger factory for each config", func(t *testing.T) {
			require.Len(t, loggerNames, qty)
		})
		t.Run("should call webhook factory for each config that needs it", func(t *testing.T) {
			require.Len(t, webhooks, 17) // we have 17 notifiers that support webhook
		})
		t.Run("should call email factory for each config that needs it", func(t *testing.T) {
			require.Len(t, emails, 1) // we have only email notifier that needs sender
		})
	})
	t.Run("should return errors if webhook factory fails", func(t *testing.T) {
		fullCfg, _ := getFullConfig(t)
		calls := 0
		failingFactory := func(n receivers.Metadata) (receivers.WebhookSender, error) {
			calls++
			return nil, errors.New("bad-test")
		}

		integrations, err := BuildReceiverIntegrations(fullCfg, tmpl, imageProvider, loggerFactory, failingFactory, emailFactory, orgID, version)

		require.Empty(t, integrations)
		require.NotNil(t, err)
		require.ErrorContains(t, err, "bad-test")
		require.Greater(t, calls, 0)
	})
	t.Run("should return errors if email factory fails", func(t *testing.T) {
		fullCfg, _ := getFullConfig(t)
		calls := 0
		failingFactory := func(n receivers.Metadata) (receivers.EmailSender, error) {
			calls++
			return nil, errors.New("bad-test")
		}

		integrations, err := BuildReceiverIntegrations(fullCfg, tmpl, imageProvider, loggerFactory, webhookFactory, failingFactory, orgID, version)

		require.Empty(t, integrations)
		require.NotNil(t, err)
		require.ErrorContains(t, err, "bad-test")
		require.Greater(t, calls, 0)
	})
	t.Run("should not produce any integration if config is empty", func(t *testing.T) {
		cfg := GrafanaReceiverConfig{Name: "test"}

		integrations, err := BuildReceiverIntegrations(cfg, tmpl, imageProvider, loggerFactory, webhookFactory, emailFactory, orgID, version)

		require.NoError(t, err)
		require.Empty(t, integrations)
	})
}
