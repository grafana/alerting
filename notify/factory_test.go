package notify

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/config"
	commoncfg "github.com/prometheus/common/config"

	"github.com/prometheus/alertmanager/notify"

	"github.com/grafana/alerting/definition"
	"github.com/grafana/alerting/http"
	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/notifytest"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestBuildReceiverIntegrations(t *testing.T) {
	var orgID = rand.Int63()
	var version = fmt.Sprintf("Grafana v%d", rand.Uint32())
	imageProvider := &images.URLProvider{}
	tmpl := templates.ForTests(t)

	emailService := receivers.MockNotificationService()

	noopWrapper := func(_ string, n Notifier) Notifier {
		return n
	}

	getFullConfig := func(t *testing.T) (GrafanaReceiverConfig, int) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for _, cfg := range notifytest.AllKnownV1ConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(""))
		}
		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, GetDecryptedValueFnForTesting)
		require.NoError(t, err)
		return parsed, len(recCfg.Integrations)
	}

	t.Run("should build all supported notifiers", func(t *testing.T) {
		fullCfg, qty := getFullConfig(t)

		wrapped := 0
		notifyWrapper := func(_ string, n Notifier) Notifier {
			wrapped++
			return n
		}

		integrations, err := BuildGrafanaReceiverIntegrations(fullCfg, tmpl, imageProvider, log.NewNopLogger(), emailService, notifyWrapper, orgID, version, nil)
		require.NoError(t, err)

		require.Len(t, integrations, qty)

		t.Run("should call notify wrapper for each config", func(t *testing.T) {
			require.Equal(t, qty, wrapped)
		})
		t.Run("should use custom dial context", func(t *testing.T) {
			customDialError := fmt.Errorf("custom dial function error")
			clientOpts := []http.ClientOption{
				http.WithUserAgent("Grafana-test"),
				http.WithDialer(net.Dialer{
					// Override the Resolver so that configurations with invalid hostnames also return
					// "custom dial function error" instead of "no such host".
					Resolver: &net.Resolver{
						Dial: func(_ context.Context, _, _ string) (net.Conn, error) {
							return nil, customDialError
						},
					},
					Control: func(_, _ string, _ syscall.RawConn) error {
						return customDialError
					},
				}),
			}

			integrations, err := BuildGrafanaReceiverIntegrations(fullCfg, tmpl, imageProvider, log.NewNopLogger(), emailService, notifyWrapper, orgID, version, nil, clientOpts...)
			require.NoError(t, err)

			require.Len(t, integrations, qty)
			for _, integration := range integrations {
				if integration.Name() == "email" {
					continue // skip email integration, it is not using webhook sender.
				}
				t.Run(integration.Name(), func(t *testing.T) {
					if integration.Name() == "mqtt" {
						t.Skip() // TODO: mqtt integration does not support custom dialer yet.
					}
					if integration.Name() == "sns" {
						t.Skip() // TODO: sns integration does not support custom dialer yet.
					}
					if integration.Name() == "slack" {
						t.Skip() // TODO: slack integration does not support custom dialer yet.
					}
					if integration.Name() == "prometheus-alertmanager" {
						t.Skip() // TODO: prometheus-alertmanager integration does not support custom dialer yet.
					}
					alert := newTestAlert(nil, time.Now(), time.Now())

					ctx := context.Background()
					ctx = notify.WithGroupKey(ctx, fmt.Sprintf("%s-%s-%d", integration.Name(), alert.Labels.Fingerprint(), time.Now().Unix()))
					ctx = notify.WithGroupLabels(ctx, alert.Labels)
					ctx = notify.WithReceiverName(ctx, integration.String())
					_, err := integration.Notify(ctx, &alert)
					require.Error(t, err)
					require.ErrorContains(t, err, customDialError.Error())
				})
			}
		})
	})
	t.Run("should not produce any integration if config is empty", func(t *testing.T) {
		cfg := GrafanaReceiverConfig{Name: "test"}

		integrations, err := BuildGrafanaReceiverIntegrations(cfg, tmpl, imageProvider, log.NewNopLogger(), emailService, noopWrapper, orgID, version, nil)
		require.NoError(t, err)
		require.Empty(t, integrations)
	})
}

func TestBuildReceiversIntegrations(t *testing.T) {
	var orgID = rand.Int63()
	var version = fmt.Sprintf("Grafana v%d", rand.Uint32())
	imageProvider := &images.URLProvider{}
	tmpl, err := templates.NewFactory(nil, log.NewNopLogger(), "http://localhost", "grafana")
	require.NoError(t, err)
	emailService := receivers.MockNotificationService()

	t.Run("should build receivers", func(t *testing.T) {
		apiReceivers := []*APIReceiver{
			{
				ConfigReceiver: ConfigReceiver{
					Name: "test1",
					WebhookConfigs: []*config.WebhookConfig{
						{
							HTTPConfig: &commoncfg.DefaultHTTPClientConfig,
						},
					},
				},
			},
			{
				ConfigReceiver: ConfigReceiver{
					Name: "test2",
				},
				ReceiverConfig: models.ReceiverConfig{
					Integrations: []*models.IntegrationConfig{
						notifytest.AllKnownV1ConfigsForTesting["email"].GetRawNotifierConfig("test2"),
					},
				},
			},
		}

		actual, err := BuildReceiversIntegrations(
			orgID,
			apiReceivers,
			tmpl,
			imageProvider,
			NoopDecrypt,
			DecodeSecretsFromBase64,
			emailService,
			nil,
			func(_ string, n notify.Notifier) notify.Notifier {
				return n
			},
			version,
			log.NewNopLogger(),
			nil,
		)
		require.NoError(t, err)
		require.Contains(t, actual, "test1")
		require.Equal(t, "webhook[0]", actual["test1"][0].String())
		require.Contains(t, actual, "test2")
		require.Equal(t, "email[0]", actual["test2"][0].String())
	})

	t.Run("should ignore duplicates", func(t *testing.T) {
		apiReceivers := []*APIReceiver{
			{
				ConfigReceiver: ConfigReceiver{
					Name: "test",
				},
				ReceiverConfig: models.ReceiverConfig{
					Integrations: []*models.IntegrationConfig{
						notifytest.AllKnownV1ConfigsForTesting["email"].GetRawNotifierConfig("test"),
					},
				},
			},
			{
				ConfigReceiver: ConfigReceiver{
					Name: "test",
				},
				ReceiverConfig: models.ReceiverConfig{
					Integrations: []*models.IntegrationConfig{
						notifytest.AllKnownV1ConfigsForTesting["webhook"].GetRawNotifierConfig("test"),
					},
				},
			},
		}

		actual, err := BuildReceiversIntegrations(
			orgID,
			apiReceivers,
			tmpl,
			imageProvider,
			NoopDecrypt,
			DecodeSecretsFromBase64,
			emailService,
			nil,
			func(_ string, n notify.Notifier) notify.Notifier {
				return n
			},
			version,
			log.NewNopLogger(),
			nil,
		)
		require.NoError(t, err)
		require.Contains(t, actual, "test")
		integrations := actual["test"]
		require.Len(t, integrations, 1)
		require.Equal(t, "webhook[0]", integrations[0].String())
	})
}

func TestBuildPrometheusReceiverIntegrations(t *testing.T) {
	receiver, err := notifytest.GetMimirReceiverWithAllIntegrations(notifytest.WithTLS, notifytest.WithAuthorization, notifytest.WithOAuth2)
	require.NoError(t, err)
	err = definition.ValidateAlertmanagerConfig(receiver)
	require.NoError(t, err)
	tmpl, err := templates.NewFactory(nil, log.NewNopLogger(), "http://localhost", "1")
	require.NoError(t, err)
	integrations, err := BuildPrometheusReceiverIntegrations(receiver, tmpl, nil, log.NewNopLogger(), NoWrap, nil)
	require.NoError(t, err)
	require.Len(t, integrations, 14)
}
