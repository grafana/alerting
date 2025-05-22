package notify

import (
	"bytes"
	"sync"
	tmpltext "text/template"

	"github.com/prometheus/alertmanager/template"

	"github.com/grafana/alerting/templates"
)

type TestTemplatesConfigBodyParams struct {
	// Alerts to use as data when testing the template.
	Alerts []*PostableAlert

	// Template string to test.
	Template string

	// Name of the template.
	Name string

	// Kind of template to test. Default is Grafana
	Kind templates.Kind
}

type TestTemplatesResults struct {
	Results []TestTemplatesResult      `json:"results"`
	Errors  []TestTemplatesErrorResult `json:"errors"`
}

type TestTemplatesResult struct {
	// Name of the associated template definition for this result.
	Name string `json:"name"`

	// Interpolated value of the template.
	Text string `json:"text"`

	// Scope that was successfully used to interpolate the template. If the root scope "." fails, more specific
	// scopes will be tried, such as ".Alerts', or ".Alert".
	Scope TemplateScope `json:"scope"`
}

type TestTemplatesErrorResult struct {
	// Name of the associated template for this error. Will be empty if the Kind is "invalid_template".
	Name string `json:"name"`

	// Kind of template error that occurred.
	Kind TemplateErrorKind `json:"kind"`

	// Error cause.
	Error string `json:"error"`
}

type TemplateErrorKind string

const (
	InvalidTemplate TemplateErrorKind = "invalid_template"
	ExecutionError  TemplateErrorKind = "execution_error"
)

const (
	DefaultReceiverName    = "TestReceiver"
	DefaultGroupLabel      = "group_label"
	DefaultGroupLabelValue = "group_label_value"
)

// TemplateScope is the scope used to interpolate the template when testing.
type TemplateScope string

const (
	rootScope   TemplateScope = "."
	alertsScope TemplateScope = ".Alerts"
	alertScope  TemplateScope = ".Alert"
)

// Data returns the template data to be used with the given scope.
func (s TemplateScope) Data(data *templates.ExtendedData) any {
	switch s {
	case rootScope:
		return data
	case alertsScope:
		return data.Alerts
	case alertScope:
		if len(data.Alerts) > 0 {
			return data.Alerts[0]
		}
	}
	return nil
}

func (am *GrafanaAlertmanager) GetTemplate(kind templates.Kind) (*template.Template, error) {
	am.reloadConfigMtx.RLock()
	defer am.reloadConfigMtx.RUnlock()
	return am.templates.Get(kind)
}

// testTemplateScopes tests the given template with the root scope. If the root scope fails, it tries
// other more specific scopes, such as ".Alerts" or ".Alert".
// If none of the more specific scopes work either, the original error is returned.
func testTemplateScopes(newTextTmpl *tmpltext.Template, def string, data *templates.ExtendedData) (string, TemplateScope, error) {
	var buf bytes.Buffer
	defaultErr := newTextTmpl.ExecuteTemplate(&buf, def, data)
	if defaultErr == nil {
		return buf.String(), rootScope, nil
	}

	// Before returning this error, we try others scopes to see if the error is due to the template being intended
	// to be used with a specific scope, such as ".Alerts" or ".Alert". If none of these scopes work, we return
	// the original error.
	// This is a fairly brute force approach, but it's the best we can do without heuristics or asking the
	// caller to provide the correct scope.
	for _, scope := range []TemplateScope{alertsScope, alertScope} {
		var buf bytes.Buffer
		err := newTextTmpl.ExecuteTemplate(&buf, def, scope.Data(data))
		if err == nil {
			return buf.String(), scope, nil
		}
	}

	return "", rootScope, defaultErr
}

func newTemplatesCache(f *templates.Factory) *templatesCache {
	return &templatesCache{
		factory: f,
		m:       map[templates.Kind]*templates.Template{},
	}
}

// templatesCache is responsible for managing template instances grouped by their kind.
// It utilizes a Factory to create templates when requested.
// Templates are cached in-memory to avoid redundant creation.
// Access is synchronized using a mutex to ensure thread-safety.
type templatesCache struct {
	factory *templates.Factory
	m       map[templates.Kind]*templates.Template
	mtx     sync.Mutex
}

func (g *templatesCache) Get(kind templates.Kind) (*template.Template, error) {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	if t, ok := g.m[kind]; ok {
		return t, nil
	}
	t, err := g.factory.NewTemplate(kind)
	if err != nil {
		return nil, err
	}
	g.m[kind] = t
	return t, nil
}
