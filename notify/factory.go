package notify

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	"github.com/prometheus/alertmanager/notify"

	"github.com/grafana/alerting/http"
	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/nfstatus"
	"github.com/grafana/alerting/receivers"
	alertmanager "github.com/grafana/alerting/receivers/alertmanager/v1"
	dingding "github.com/grafana/alerting/receivers/dingding/v1"
	discord "github.com/grafana/alerting/receivers/discord/v1"
	email "github.com/grafana/alerting/receivers/email/v1"
	googlechat "github.com/grafana/alerting/receivers/googlechat/v1"
	jira "github.com/grafana/alerting/receivers/jira/v1"
	kafka "github.com/grafana/alerting/receivers/kafka/v1"
	line "github.com/grafana/alerting/receivers/line/v1"
	mqtt "github.com/grafana/alerting/receivers/mqtt/v1"
	oncall "github.com/grafana/alerting/receivers/oncall/v1"
	opsgenie "github.com/grafana/alerting/receivers/opsgenie/v1"
	pagerduty "github.com/grafana/alerting/receivers/pagerduty/v1"
	pushover "github.com/grafana/alerting/receivers/pushover/v1"
	"github.com/grafana/alerting/receivers/schema"
	sensugo "github.com/grafana/alerting/receivers/sensugo/v1"
	slack "github.com/grafana/alerting/receivers/slack/v1"
	sns "github.com/grafana/alerting/receivers/sns/v1"
	teams "github.com/grafana/alerting/receivers/teams/v1"
	telegram "github.com/grafana/alerting/receivers/telegram/v1"
	threema "github.com/grafana/alerting/receivers/threema/v1"
	victorops "github.com/grafana/alerting/receivers/victorops/v1"
	webex "github.com/grafana/alerting/receivers/webex/v1"
	webhook "github.com/grafana/alerting/receivers/webhook/v1"
	wecom "github.com/grafana/alerting/receivers/wecom/v1"
	"github.com/grafana/alerting/templates"
)

type WrapNotifierFunc func(integrationName string, notifier nfstatus.Notifier) nfstatus.Notifier

var NoWrap WrapNotifierFunc = func(_ string, notifier nfstatus.Notifier) nfstatus.Notifier { return notifier }

// BuildGrafanaReceiverIntegrations creates integrations for each configured notification channel in GrafanaReceiverConfig.
// It returns a slice of Integration objects, one for each notification channel, along with any errors that occurred.
func BuildGrafanaReceiverIntegrations(
	receiver GrafanaReceiverConfig,
	tmpl *templates.Template,
	img images.Provider,
	logger log.Logger,
	emailSender receivers.EmailSender,
	wrapNotifier WrapNotifierFunc,
	orgID int64,
	version string,
	notificationHistorian nfstatus.NotificationHistorian,
	httpClientOptions ...http.ClientOption,
) ([]*Integration, error) {
	type notificationChannel interface {
		notify.Notifier
		notify.ResolvedSender
	}
	var (
		integrations []*Integration
		errs         error
		ci           = func(idx int, cfg receivers.Metadata, httpClientConfig *http.HTTPClientConfig, newInt func(cli *http.Client) notificationChannel) {
			client, err := http.NewClient(httpClientConfig, httpClientOptions...)
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("failed to create HTTP client for %q notifier %q (UID: %q): %w", cfg.Type, cfg.Name, cfg.UID, err))
				return
			}
			n := newInt(client)
			notify := wrapNotifier(cfg.Name, nfstatus.NewNotifierAdapter(n))
			i := NewIntegration(notify, n, string(cfg.Type), idx, cfg.Name, notificationHistorian, logger)
			integrations = append(integrations, i)
		}
	)
	// Range through each notification channel in the receiver and create an integration for it.
	for i, cfg := range receiver.AlertmanagerConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(_ *http.Client) notificationChannel {
			return alertmanager.New(cfg.Settings, cfg.Metadata, img, logger)
		})
	}
	for i, cfg := range receiver.DingdingConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return dingding.New(cfg.Settings, cfg.Metadata, tmpl, cli, logger)
		})
	}
	for i, cfg := range receiver.DiscordConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return discord.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, version)
		})
	}
	for i, cfg := range receiver.EmailConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(_ *http.Client) notificationChannel {
			return email.New(cfg.Settings, cfg.Metadata, tmpl, emailSender, img, logger)
		})
	}
	for i, cfg := range receiver.GooglechatConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return googlechat.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, version)
		})
	}
	for i, cfg := range receiver.JiraConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return jira.New(cfg.Settings, cfg.Metadata, tmpl, http.NewForkedSender(cli), logger)
		})
	}
	for i, cfg := range receiver.KafkaConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return kafka.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.LineConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return line.New(cfg.Settings, cfg.Metadata, tmpl, cli, logger)
		})
	}
	for i, cfg := range receiver.MqttConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(_ *http.Client) notificationChannel {
			return mqtt.New(cfg.Settings, cfg.Metadata, tmpl, logger, nil)
		})
	}
	for i, cfg := range receiver.OnCallConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return oncall.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, orgID)
		})
	}
	for i, cfg := range receiver.OpsgenieConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return opsgenie.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.PagerdutyConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return pagerduty.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.PushoverConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return pushover.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.SensugoConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return sensugo.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.SNSConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(_ *http.Client) notificationChannel {
			return sns.New(cfg.Settings, cfg.Metadata, tmpl, logger)
		})
	}
	for i, cfg := range receiver.SlackConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return slack.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, version)
		})
	}
	for i, cfg := range receiver.TeamsConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return teams.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.TelegramConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return telegram.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.ThreemaConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return threema.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger)
		})
	}
	for i, cfg := range receiver.VictoropsConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return victorops.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, version)
		})
	}
	for i, cfg := range receiver.WebhookConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return webhook.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, orgID)
		})
	}
	for i, cfg := range receiver.WecomConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return wecom.New(cfg.Settings, cfg.Metadata, tmpl, cli, logger)
		})
	}
	for i, cfg := range receiver.WebexConfigs {
		ci(i, cfg.Metadata, cfg.HTTPClientConfig, func(cli *http.Client) notificationChannel {
			return webex.New(cfg.Settings, cfg.Metadata, tmpl, cli, img, logger, orgID)
		})
	}
	return integrations, errs
}

// BuildReceiversIntegrations builds integrations for the provided API receivers and returns them mapped by receiver name.
// It ensures uniqueness of receivers by the name, overwriting duplicates and logs warnings.
// Returns an error if any integration fails during its construction.
func BuildReceiversIntegrations(
	tenantID int64,
	receivers []models.ReceiverConfig,
	templ TemplatesProvider,
	images images.Provider,
	decryptFn GetDecryptedValueFn,
	decodeFn DecodeSecretsFn,
	emailSender receivers.EmailSender,
	httpClientOptions []http.ClientOption,
	notifierFunc WrapNotifierFunc,
	version string,
	logger log.Logger,
	notificationHistorian nfstatus.NotificationHistorian,
	useManifestBuilder bool,
) (map[string][]*Integration, error) {
	nameToReceiver := make(map[string]models.ReceiverConfig, len(receivers))
	for _, receiver := range receivers {
		if existing, ok := nameToReceiver[receiver.Name]; ok {
			itypes := make([]string, 0, len(existing.Integrations))
			for _, i := range existing.Integrations {
				itypes = append(itypes, string(i.Type))
			}
			level.Warn(logger).Log("msg", "receiver with same name is defined multiple times. Only the last one will be used", "receiver_name", receiver.Name, "overwritten_integrations", itypes)
		}
		nameToReceiver[receiver.Name] = receiver
	}

	integrationsMap := make(map[string][]*Integration, len(receivers))
	for name, apiReceiver := range nameToReceiver {
		var integrations []*Integration
		var err error
		if useManifestBuilder {
			integrations, err = BuildReceiverIntegrationsWithManifests(tenantID, apiReceiver, templ, images, decryptFn, decodeFn, emailSender, httpClientOptions, notifierFunc, version, logger, notificationHistorian)
		} else {
			integrations, err = BuildReceiverIntegrations(tenantID, apiReceiver, templ, images, decryptFn, decodeFn, emailSender, httpClientOptions, notifierFunc, version, logger, notificationHistorian)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to build receiver %s: %w", name, err)
		}
		integrationsMap[name] = integrations
	}
	return integrationsMap, nil
}

// BuildReceiverIntegrations builds integrations for the provided API receiver and returns them.
// It supports both Prometheus and Grafana integrations and ensures that both of them use only templates dedicated for the kind.
func BuildReceiverIntegrations(
	tenantID int64,
	receiver models.ReceiverConfig,
	tmpls TemplatesProvider,
	images images.Provider,
	decryptFn GetDecryptedValueFn,
	decodeFn DecodeSecretsFn,
	emailSender receivers.EmailSender,
	httpClientOptions []http.ClientOption,
	wrapNotifierFunc WrapNotifierFunc,
	version string,
	logger log.Logger,
	notificationHistorian nfstatus.NotificationHistorian,
) ([]*Integration, error) {
	var integrations []*Integration
	if len(receiver.Integrations) > 0 {
		receiverCfg, err := BuildReceiverConfiguration(context.Background(), receiver, decodeFn, decryptFn)
		if err != nil {
			return nil, err
		}
		tmpl, err := tmpls.GetTemplate(templates.GrafanaKind)
		if err != nil {
			return nil, err
		}
		integrations, err = BuildGrafanaReceiverIntegrations(
			receiverCfg,
			tmpl,
			images,
			logger,
			emailSender,
			wrapNotifierFunc,
			tenantID,
			version,
			notificationHistorian,
			httpClientOptions...,
		)
		if err != nil {
			return nil, err
		}
	}
	return integrations, nil
}

// BuildReceiverIntegrationsWithManifests builds integrations for the provided API receiver using
// the manifest-based factory for v1 (Grafana) integrations instead of the typed config switch.
// Unlike BuildGrafanaReceiverIntegrations, this function is fail-fast: it returns on the first
// error without returning partial results.
func BuildReceiverIntegrationsWithManifests(
	tenantID int64,
	receiver models.ReceiverConfig,
	tmpls TemplatesProvider,
	images images.Provider,
	decryptFn GetDecryptedValueFn,
	decodeFn DecodeSecretsFn,
	emailSender receivers.EmailSender,
	httpClientOptions []http.ClientOption,
	wrapNotifierFunc WrapNotifierFunc,
	version string,
	logger log.Logger,
	notificationHistorian nfstatus.NotificationHistorian,
) ([]*Integration, error) {
	var integrations []*Integration
	if len(receiver.Integrations) > 0 {
		opts := receivers.NotifierOpts{
			Images:         images,
			Logger:         logger,
			EmailSender:    emailSender,
			OrgID:          tenantID,
			GrafanaVersion: version,
			HttpOpts:       http.ToHTTPClientOption(httpClientOptions...),
		}
		// Track per-type indices so that integrations of the same type get consecutive indices (0, 1, 2…).
		// This matches BuildGrafanaReceiverIntegrations behavior and preserves notification log management (see createReceiverStage).
		typeCounters := make(map[schema.IntegrationType]int)
		for _, cfg := range receiver.Integrations {
			idx := typeCounters[cfg.Type]
			typeCounters[cfg.Type]++

			kind := templates.GrafanaKind
			if cfg.Version != schema.V1 {
				kind = templates.MimirKind
			}
			tmpl, err := tmpls.GetTemplate(kind)
			if err != nil {
				return nil, err
			}

			meta := receivers.Metadata{
				Index:                 idx,
				UID:                   cfg.UID,
				Name:                  cfg.Name,
				Type:                  cfg.Type,
				Version:               cfg.Version,
				DisableResolveMessage: cfg.DisableResolveMessage,
			}

			secureSettings, err := decodeFn(cfg.SecureSettings)
			if err != nil {
				return nil, fmt.Errorf("failed to decode secure settings for %q (UID: %q): %w", cfg.Name, cfg.UID, err)
			}
			decrypt := receivers.DecryptFunc(func(key string, fallback string) (string, bool) {
				if _, ok := secureSettings[key]; !ok {
					return fallback, false
				}
				return decryptFn(context.Background(), secureSettings, key, fallback), true
			})

			opts := opts
			opts.Template = tmpl
			// v0 do not use sender. Instead, each v0 factory creates its own HTTP client internally,
			// combining the shared httpClientOptions (passed as variadic args to New()) with the
			// per-integration HTTP config embedded in the typed config struct.
			if cfg.Version == schema.V1 {
				// TODO refactor this down to config factory, and use the same approach as in V0
				httpClientConfig, err := parseHTTPConfig(cfg, decrypt)
				if err != nil {
					return nil, fmt.Errorf("failed to parse HTTP config for %q (UID: %q): %w", cfg.Name, cfg.UID, err)
				}
				client, err := http.NewClient(httpClientConfig, httpClientOptions...)
				if err != nil {
					return nil, fmt.Errorf("failed to create HTTP client for %q (UID: %q): %w", cfg.Name, cfg.UID, err)
				}
				// Jira's getIssueTransitionByName uses GET, which Client.SendWebhook rejects (POST/PUT only).
				// ForkedSender handles GET by bypassing Client.SendWebhook and making the request directly.
				// TODO remove it once the GET is supported by the client
				if cfg.Type == schema.JiraType {
					opts.Sender = http.NewForkedSender(client)
				} else {
					opts.Sender = client
				}
			}

			factory, ok := GetFactoryForIntegrationVersion(cfg.Type, cfg.Version)
			if !ok {
				return nil, fmt.Errorf("invalid integration type or version: %s %s", cfg.Type, cfg.Version)
			}
			n, err := factory.NewNotifier(cfg.Settings, decrypt, meta, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to build notifier for %q (UID: %q): %w", cfg.Name, cfg.UID, err)
			}
			wrapped := wrapNotifierFunc(cfg.Name, nfstatus.NewNotifierAdapter(n))
			integrations = append(integrations, NewIntegration(wrapped, n, string(cfg.Type), idx, cfg.Name, notificationHistorian, logger))
		}
	}
	return integrations, nil
}
