package templates

import (
	"net/url"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/alertmanager/template"
)

// Factory is a factory that can be used to create templates of specific kind.
type Factory struct {
	templates   map[Kind][]TemplateDefinition
	externalURL *url.URL
}

// NewTemplate creates a new template of the given kind. If Kind is not known, GrafanaKind automatically assumed
func (tp *Factory) NewTemplate(kind Kind, options ...template.Option) (*Template, error) {
	if !IsKnownKind(kind) {
		kind = GrafanaKind
	}
	definitions := tp.templates[kind]
	content := defaultTemplatesPerKind(kind)
	for _, def := range definitions { // TODO sort the list by name?
		content = append(content, def.Template)
	}
	t, err := fromContent(content, append(defaultOptionsPerKind(kind), options...)...)
	if err != nil {
		return nil, err
	}
	if tp.externalURL != nil {
		t.ExternalURL = new(url.URL)
		*t.ExternalURL = *tp.externalURL
	}
	return t, nil
}

// NewFactory creates a new template provider. Accepts list of user-defined templates that are added to the kind's default templates.
// Returns error if externalURL is not a valid URL. If TemplateDefinition.Kind is not known, GrafanaKind automatically assumed
func NewFactory(t []TemplateDefinition, logger log.Logger, externalURL string) (*Factory, error) {
	extURL, err := url.Parse(externalURL)
	if err != nil {
		return nil, err
	}
	type seenKey struct {
		Name string
		Type Kind
	}
	seen := map[seenKey]struct{}{}
	byType := map[Kind][]TemplateDefinition{}

	for _, def := range t {
		if !IsKnownKind(def.Kind) {
			level.Warn(logger).Log("msg", "template without kind is defined, assuming Grafana template", "template_name", def.Name)
			def.Kind = GrafanaKind
		}
		if _, ok := seen[seenKey{Name: def.Name, Type: def.Kind}]; ok {
			level.Warn(logger).Log("msg", "template with same name is defined multiple times, skipping...", "template_name", def.Name, "template_type", def.Kind)
			continue
		}
		byType[def.Kind] = append(byType[def.Kind], def)
		seen[seenKey{Name: def.Name, Type: def.Kind}] = struct{}{}
	}
	provider := &Factory{
		templates:   byType,
		externalURL: extURL,
	}
	return provider, nil
}
