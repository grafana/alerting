package receivers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/prometheus/alertmanager/template"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
)

// GetDecryptedValueFn is a function that returns the decrypted value of
// the given key. If the key is not present, then it returns the fallback value.
type GetDecryptedValueFn func(ctx context.Context, sjd map[string][]byte, key string, fallback string) string

type DecryptFunc func(key string, fallback string) string

type NotificationSender interface {
	WebhookSender
	EmailSender
}

type NotificationChannelConfig struct {
	OrgID                 int64             // only used internally
	UID                   string            `json:"uid"`
	Name                  string            `json:"name"`
	Type                  string            `json:"type"`
	DisableResolveMessage bool              `json:"disableResolveMessage"`
	Settings              json.RawMessage   `json:"settings"`
	SecureSettings        map[string][]byte `json:"secureSettings"`
}

// NotifierInfo contains metadata of the notifier. Name, UID, Type, etc
type NotifierInfo struct {
	UID                   string
	Name                  string
	Type                  string
	DisableResolveMessage bool
}

type FactoryConfig struct {
	Config *NotificationChannelConfig
	// Used by some receivers to include as part of the source
	GrafanaBuildVersion string
	NotificationService NotificationSender
	DecryptFunc         GetDecryptedValueFn
	ImageStore          images.ImageStore
	// Used to retrieve image URLs for messages, or data for uploads.
	Template *template.Template
	Logger   logging.Logger
}

func (fc *FactoryConfig) Decrypt(key, fallback string) string {
	return fc.DecryptFunc(context.Background(), fc.Config.SecureSettings, key, fallback)
}

func NewFactoryConfig(config *NotificationChannelConfig, notificationService NotificationSender,
	decryptFunc GetDecryptedValueFn, template *template.Template, imageStore images.ImageStore, loggerFactory logging.LoggerFactory, buildVersion string) (FactoryConfig, error) {
	if config.Settings == nil {
		return FactoryConfig{}, errors.New("no settings supplied")
	}
	// not all receivers do need secure settings, we still might interact with
	// them, so we make sure they are never nil
	if config.SecureSettings == nil {
		config.SecureSettings = map[string][]byte{}
	}

	if imageStore == nil {
		imageStore = &images.UnavailableImageStore{}
	}
	return FactoryConfig{
		Config:              config,
		NotificationService: notificationService,
		GrafanaBuildVersion: buildVersion,
		DecryptFunc:         decryptFunc,
		Template:            template,
		ImageStore:          imageStore,
		Logger:              loggerFactory("ngalert.notifier." + config.Type),
	}, nil
}
