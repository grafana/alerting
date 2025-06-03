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

	"github.com/prometheus/alertmanager/notify"

	"github.com/grafana/alerting/http"
	"github.com/grafana/alerting/images"
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
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
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

		integrations := BuildGrafanaReceiverIntegrations(fullCfg, tmpl, imageProvider, log.NewNopLogger(), emailService, notifyWrapper, orgID, version)

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

			integrations := BuildGrafanaReceiverIntegrations(fullCfg, tmpl, imageProvider, log.NewNopLogger(), emailService, notifyWrapper, orgID, version, clientOpts...)

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
	t.Run("should not produce any integration if config is empty", func(t *testing.T) {
		cfg := GrafanaReceiverConfig{Name: "test"}

		integrations := BuildGrafanaReceiverIntegrations(cfg, tmpl, imageProvider, log.NewNopLogger(), emailService, noopWrapper, orgID, version)
		require.Empty(t, integrations)
	})
}
