package receivers

import (
	"context"
	"encoding/json"
	"errors"

	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/alertmanager/template"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
)

// GetDecryptedValueFn is a function that returns the decrypted value of
// the given key. If the key is not present, then it returns the fallback value.
type GetDecryptedValueFn func(ctx context.Context, sjd map[string][]byte, key string, fallback string) string

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

type FactoryConfig struct {
	Config *NotificationChannelConfig
	// Used by some receivers to include as part of the source
	GrafanaBuildVersion string
	NotificationService NotificationSender
	ImageStore          images.ImageStore
	// Used to retrieve image URLs for messages, or data for uploads.
	Template   *template.Template
	Logger     logging.Logger
	Marshaller jsoniter.API
}

func NewFactoryConfig(config *NotificationChannelConfig, notificationService NotificationSender,
	decryptFunc GetDecryptedValueFn, template *template.Template, imageStore images.ImageStore, loggerFactory logging.LoggerFactory, buildVersion string) (FactoryConfig, error) {
	if config.Settings == nil {
		return FactoryConfig{}, errors.New("no settings supplied")
	}
	if imageStore == nil {
		imageStore = &images.UnavailableImageStore{}
	}
	return FactoryConfig{
		Config:              config,
		NotificationService: notificationService,
		GrafanaBuildVersion: buildVersion,
		Template:            template,
		ImageStore:          imageStore,
		Logger:              loggerFactory("ngalert.notifier." + config.Type),
		Marshaller:          CreateMarshallerWithSecretsDecrypt(decryptFunc, config.SecureSettings),
	}, nil
}
