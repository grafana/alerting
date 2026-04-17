package receivers

import (
	"encoding/json"
	"fmt"

	"github.com/go-kit/log"
	commoncfg "github.com/prometheus/common/config"

	"github.com/prometheus/alertmanager/notify"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/templates"
)

// NotifierOpts bundles runtime dependencies for constructing any notifier.
type NotifierOpts struct {
	Template       *templates.Template
	Images         images.Provider
	Logger         log.Logger
	EmailSender    EmailSender
	Sender         WebhookSender
	OrgID          int64
	GrafanaVersion string
	HttpOpts       []commoncfg.HTTPClientOption
}

// NotificationChannel is the interface that all notifiers must satisfy.
type NotificationChannel interface {
	notify.Notifier
	notify.ResolvedSender
}

type ValidateIntegrationFunc func(json.RawMessage, DecryptFunc) error
type NewNotifierFunc func(raw json.RawMessage, decryptFn DecryptFunc, meta Metadata, opts NotifierOpts) (NotificationChannel, error)

type IntegrationVersionFactory struct {
	Version        schema.Version
	Type           schema.IntegrationType
	ValidateConfig ValidateIntegrationFunc
	NewNotifier    NewNotifierFunc
}

type Manifest struct {
	schema.IntegrationTypeSchema
	factories []IntegrationVersionFactory
}

func NewManifest(s schema.IntegrationTypeSchema, factories ...IntegrationVersionFactory) Manifest {
	factoryVersions := make(map[schema.Version]struct{}, len(factories))
	for _, f := range factories {
		if _, ok := s.GetVersion(f.Version); !ok {
			panic(fmt.Sprintf("factory version %s not found in schema for %s", f.Version, s.Type))
		}
		if _, ok := factoryVersions[f.Version]; ok {
			panic(fmt.Sprintf("duplicate factory version %s for %s", f.Version, s.Type))
		}
		factoryVersions[f.Version] = struct{}{}
	}
	for _, v := range s.Versions {
		if _, ok := factoryVersions[v.Version]; !ok {
			panic(fmt.Sprintf("schema version %s has no factory for %s", v.Version, s.Type))
		}
	}
	return Manifest{
		IntegrationTypeSchema: s,
		factories:             factories,
	}
}

func (i Manifest) GetFactoryForVersion(version schema.Version) (IntegrationVersionFactory, bool) {
	for _, integration := range i.factories {
		if integration.Version == version {
			return integration, true
		}
	}
	return IntegrationVersionFactory{}, false
}
