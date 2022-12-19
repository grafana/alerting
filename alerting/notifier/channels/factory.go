package channels

import (
	"context"
	"errors"

	"github.com/prometheus/alertmanager/template"
)

// GetDecryptedValueFn is a function that returns the decrypted value of
// the given key. If the key is not present, then it returns the fallback value.
type GetDecryptedValueFn func(ctx context.Context, sjd map[string][]byte, key string, fallback string) string

type FactoryConfig struct {
	Config *NotificationChannelConfig
	// Used by some receivers to include as part of the source
	GrafanaBuildVersion string
	NotificationService NotificationSender
	DecryptFunc         GetDecryptedValueFn
	ImageStore          ImageStore
	// Used to retrieve image URLs for messages, or data for uploads.
	Template *template.Template
	Logger   Logger
}

func NewFactoryConfig(config *NotificationChannelConfig, notificationService NotificationSender,
	decryptFunc GetDecryptedValueFn, template *template.Template, imageStore ImageStore, loggerFactory LoggerFactory, buildVersion string) (FactoryConfig, error) {
	if config.Settings == nil {
		return FactoryConfig{}, errors.New("no settings supplied")
	}
	// not all receivers do need secure settings, we still might interact with
	// them, so we make sure they are never nil
	if config.SecureSettings == nil {
		config.SecureSettings = map[string][]byte{}
	}

	if imageStore == nil {
		imageStore = &UnavailableImageStore{}
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
