package receivers

import (
	"encoding/json"
	"fmt"
	"reflect"

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

// VersionFactory exposes the operations shared by factories with different config types.
// Typed parsing is available on the concrete IntegrationVersionFactory[T].
type VersionFactory interface {
	Type() schema.IntegrationType
	Version() schema.Version
	ConfigType() reflect.Type
	ValidateConfig(json.RawMessage, DecryptFunc) error
	NewNotifier(json.RawMessage, DecryptFunc, Metadata, NotifierOpts) (NotificationChannel, error)
}

// IntegrationVersionFactory binds a config type to its parser and notifier builder.
type IntegrationVersionFactory[T any] struct {
	integrationType schema.IntegrationType
	version         schema.Version
	parseConfig     func(json.RawMessage, DecryptFunc) (T, error)
	buildNotifier   func(T, Metadata, NotifierOpts) (NotificationChannel, error)
}

// NewIntegrationVersionFactory infers the config type from the parser and builder.
func NewIntegrationVersionFactory[T any](
	integrationType schema.IntegrationType,
	version schema.Version,
	parseConfig func(json.RawMessage, DecryptFunc) (T, error),
	buildNotifier func(T, Metadata, NotifierOpts) (NotificationChannel, error),
) IntegrationVersionFactory[T] {
	return IntegrationVersionFactory[T]{
		integrationType: integrationType,
		version:         version,
		parseConfig:     parseConfig,
		buildNotifier:   buildNotifier,
	}
}

func (f IntegrationVersionFactory[T]) Type() schema.IntegrationType { return f.integrationType }
func (f IntegrationVersionFactory[T]) Version() schema.Version      { return f.version }

// ConfigType returns the parsed config type, which may differ from the JSON wire representation.
func (f IntegrationVersionFactory[T]) ConfigType() reflect.Type { return reflect.TypeFor[T]() }

// Parse decodes, decrypts, and validates settings using the integration's config parser.
func (f IntegrationVersionFactory[T]) Parse(raw json.RawMessage, decrypt DecryptFunc) (T, error) {
	return f.parseConfig(raw, decrypt)
}

func (f IntegrationVersionFactory[T]) ValidateConfig(raw json.RawMessage, decrypt DecryptFunc) error {
	_, err := f.Parse(raw, decrypt)
	return err
}

func (f IntegrationVersionFactory[T]) NewNotifier(raw json.RawMessage, decrypt DecryptFunc, meta Metadata, opts NotifierOpts) (NotificationChannel, error) {
	cfg, err := f.Parse(raw, decrypt)
	if err != nil {
		return nil, err
	}
	return f.buildNotifier(cfg, meta, opts)
}

type Manifest struct {
	schema.IntegrationTypeSchema
	factories []VersionFactory
}

func NewManifest(s schema.IntegrationTypeSchema, factories ...VersionFactory) Manifest {
	factoryVersions := make(map[schema.Version]struct{}, len(factories))
	for _, f := range factories {
		if _, ok := s.GetVersion(f.Version()); !ok {
			panic(fmt.Sprintf("factory version %s not found in schema for %s", f.Version(), s.Type))
		}
		if _, ok := factoryVersions[f.Version()]; ok {
			panic(fmt.Sprintf("duplicate factory version %s for %s", f.Version(), s.Type))
		}
		factoryVersions[f.Version()] = struct{}{}
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

func (i Manifest) GetFactoryForVersion(version schema.Version) (VersionFactory, bool) {
	for _, integration := range i.factories {
		if integration.Version() == version {
			return integration, true
		}
	}
	return nil, false
}
