package notify

import (
	"bytes"
	"context"
	"errors"

	tmpltext "text/template"

	"github.com/go-kit/log"
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
	MaxTemplateOutputSize  = 1024 * 1024 // 1MB
)

var ErrTemplateOutputTooLarge = errors.New("template output exceeds maximum size")

// limitedWriter wraps a buffer and limits the total bytes written.
type limitedWriter struct {
	buf       *bytes.Buffer
	remaining int
}

func newLimitedWriter(maxSize int) *limitedWriter {
	return &limitedWriter{
		buf:       &bytes.Buffer{},
		remaining: maxSize,
	}
}

func (w *limitedWriter) Write(p []byte) (n int, err error) {
	if len(p) > w.remaining {
		return 0, ErrTemplateOutputTooLarge
	}
	n, err = w.buf.Write(p)
	w.remaining -= n
	return n, err
}

func (w *limitedWriter) String() string {
	return w.buf.String()
}

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

// TestTemplate tests the given template string against the given alerts. Existing templates are used to provide context for the test.
// If an existing template of the same filename as the one being tested is found, it will not be used as context.
func (am *GrafanaAlertmanager) TestTemplate(ctx context.Context, c TestTemplatesConfigBodyParams) (*TestTemplatesResults, error) {
	am.reloadConfigMtx.RLock()
	templateFactory := am.templates
	am.reloadConfigMtx.RUnlock()

	return TestTemplate(ctx, c, templateFactory, log.With(am.logger, "operation", "TestTemplate"))
}

func (am *GrafanaAlertmanager) GetTemplate(kind templates.Kind) (*template.Template, error) {
	am.reloadConfigMtx.RLock()
	defer am.reloadConfigMtx.RUnlock()
	t, err := am.templates.GetTemplate(kind)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// testTemplateScopes tests the given template with the root scope. If the root scope fails, it tries
// other more specific scopes, such as ".Alerts" or ".Alert".
// If none of the more specific scopes work either, the original error is returned.
func testTemplateScopes(newTextTmpl *tmpltext.Template, def string, data *templates.ExtendedData) (string, TemplateScope, error) {
	buf := newLimitedWriter(MaxTemplateOutputSize)
	defaultErr := newTextTmpl.ExecuteTemplate(buf, def, data)
	if defaultErr == nil {
		return buf.String(), rootScope, nil
	}

	// Before returning this error, we try others scopes to see if the error is due to the template being intended
	// to be used with a specific scope, such as ".Alerts" or ".Alert". If none of these scopes work, we return
	// the original error.
	// This is a fairly brute force approach, but it's the best we can do without heuristics or asking the
	// caller to provide the correct scope.
	for _, scope := range []TemplateScope{alertsScope, alertScope} {
		buf := newLimitedWriter(MaxTemplateOutputSize)
		err := newTextTmpl.ExecuteTemplate(buf, def, scope.Data(data))
		if err == nil {
			return buf.String(), scope, nil
		}
	}

	return "", rootScope, defaultErr
}
