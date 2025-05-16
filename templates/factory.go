package templates

import (
	"net/url"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/alertmanager/template"
)

// Factory is a factory that can be used to create templates of specific kind.
type Factory struct {
	templates   map[Kind][]string
	externalURL *url.URL
}

// NewTemplate creates a new template of the given kind.
func (tp *Factory) NewTemplate(kind Kind, options ...template.Option) (*Template, error) {
	if kind == "" {
		kind = GrafanaTemplateKind
	}
	content := tp.templates[kind]
	t, err := fromContent(append(defaultTemplatesPerKind[kind], content...), append(defaultOptionsPerKind[kind], options...)...)
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
// Returns error if externalURL is not a valid URL.
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
	byType := map[Kind][]string{}

	for _, def := range t {
		if def.Kind == "" {
			level.Warn(logger).Log("msg", "template without kind is defined, assuming Grafana template", "template_name", def.Name)
			def.Kind = GrafanaTemplateKind
		}
		if _, ok := seen[seenKey{Name: def.Name, Type: def.Kind}]; ok {
			level.Warn(logger).Log("msg", "template with same name is defined multiple times, skipping...", "template_name", def.Name, "template_type", def.Kind)
			continue
		}
		byType[def.Kind] = append(byType[def.Kind], def.Template)
		seen[seenKey{Name: def.Name, Type: def.Kind}] = struct{}{}
	}
	provider := &Factory{
		templates:   byType,
		externalURL: extURL,
	}
	return provider, nil
}
