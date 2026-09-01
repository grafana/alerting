package notify

import (
	"context"
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	"github.com/grafana/alerting/http"
	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/nfstatus"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/templates"
)

type WrapNotifierFunc func(integrationName string, notifier nfstatus.Notifier) nfstatus.Notifier

var NoWrap WrapNotifierFunc = func(_ string, notifier nfstatus.Notifier) nfstatus.Notifier { return notifier }

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
		integrations, err := BuildReceiverIntegrationsWithManifests(tenantID, apiReceiver, templ, images, decryptFn, decodeFn, emailSender, httpClientOptions, notifierFunc, version, logger, notificationHistorian)
		if err != nil {
			return nil, fmt.Errorf("failed to build receiver %s: %w", name, err)
		}
		integrationsMap[name] = integrations
	}
	return integrationsMap, nil
}

// BuildReceiverIntegrationsWithManifests builds integrations for the provided API receiver using
// the manifest-based factory. It is fail-fast: it returns on the first error without returning
// partial results.
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
		// This preserves notification log management ordering (see createReceiverStage).
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
