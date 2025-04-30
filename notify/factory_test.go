package notify

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/http"
	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
	"github.com/prometheus/alertmanager/notify"
)

func TestBuildReceiverIntegrations(t *testing.T) {
	var orgID = rand.Int63()
	var version = fmt.Sprintf("Grafana v%d", rand.Uint32())
	imageProvider := &images.URLProvider{}
	tmpl := templates.ForTests(t)

	emailFactory := func(_ receivers.Metadata) (receivers.EmailSender, error) {
		return receivers.MockNotificationService(), nil
	}
	loggerFactory := func(_ string, _ ...interface{}) logging.Logger {
		return &logging.FakeLogger{}
	}
	noopWrapper := func(_ string, n Notifier) Notifier {
		return n
	}

	getFullConfig := func(t *testing.T) (GrafanaReceiverConfig, int) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, GetDecryptedValueFnForTesting)
		require.NoError(t, err)
		return parsed, len(recCfg.Integrations)
	}

	t.Run("should build all supported notifiers", func(t *testing.T) {
		fullCfg, qty := getFullConfig(t)

		logger := func(_ string, _ ...interface{}) logging.Logger {
			return &logging.FakeLogger{}
		}

		emails := make(map[receivers.Metadata]struct{}, qty)
		em := func(n receivers.Metadata) (receivers.EmailSender, error) {
			emails[n] = struct{}{}
			return emailFactory(n)
		}

		wrapped := 0
		notifyWrapper := func(_ string, n Notifier) Notifier {
			wrapped++
			return n
		}

		integrations, err := BuildReceiverIntegrations(fullCfg, tmpl, imageProvider, logger, nil, em, notifyWrapper, orgID, version)

		require.NoError(t, err)
		require.Len(t, integrations, qty)

		t.Run("should call email factory for each config that needs it", func(t *testing.T) {
			require.Len(t, emails, 1) // we have only email notifier that needs sender
		})
		t.Run("should call notify wrapper for each config", func(t *testing.T) {
			require.Equal(t, qty, wrapped)
		})
		t.Run("should use custom dial context", func(t *testing.T) {
			customDialError := fmt.Errorf("custom dial function error")
			clientOpts := []http.ClientOption{
				http.WithUserAgent("Grafana-test"),
				http.WithDialer(net.Dialer{
					// Also override the Resolver so that configuration that doesn't use real hostnamesalso return
					// "custom dial function error" instead of "no such dns".
					Resolver: &net.Resolver{
						Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
							return nil, customDialError
						},
					},
					Control: func(network, address string, c syscall.RawConn) error {
						return customDialError
					},
				}),
			}

			integrations, err := BuildReceiverIntegrations(fullCfg, tmpl, imageProvider, logger, clientOpts, em, notifyWrapper, orgID, version)
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
					alert := newTestAlert(TestReceiversConfigBodyParams{}, time.Now(), time.Now())

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
	t.Run("should return errors if email factory fails", func(t *testing.T) {
		fullCfg, _ := getFullConfig(t)
		calls := 0
		failingFactory := func(_ receivers.Metadata) (receivers.EmailSender, error) {
			calls++
			return nil, errors.New("bad-test")
		}

		integrations, err := BuildReceiverIntegrations(fullCfg, tmpl, imageProvider, loggerFactory, nil, failingFactory, noopWrapper, orgID, version)

		require.Empty(t, integrations)
		require.NotNil(t, err)
		require.ErrorContains(t, err, "bad-test")
		require.Greater(t, calls, 0)
	})
	t.Run("should not produce any integration if config is empty", func(t *testing.T) {
		cfg := GrafanaReceiverConfig{Name: "test"}

		integrations, err := BuildReceiverIntegrations(cfg, tmpl, imageProvider, loggerFactory, nil, emailFactory, noopWrapper, orgID, version)

		require.NoError(t, err)
		require.Empty(t, integrations)
	})
}
