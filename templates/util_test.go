package templates

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindTemplates(t *testing.T) {
	tests := []struct {
		name     string
		tmplName string
		tmplText string
		expected []string
	}{{
		name:     "returns template name for template text with range action",
		tmplName: "foo",
		tmplText: `{{ range .Alerts }}{{ end }}`,
		expected: []string{"foo"},
	}, {
		name:     "returns name of nested template for template text with define action",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range .Alerts }}{{ end }}{{ end }}`,
		expected: []string{"bar"},
	}, {
		name:     "returns name of all nested templates for template text with define actions",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range .Alerts }}{{ end }}{{ end }}{{ define "baz" }}{{ range .Alerts }}{{ end }}{{ end }}`,
		expected: []string{"bar", "baz"},
	}, {
		name:     "returns name of both nested templates when nested template has the same name as the template",
		tmplName: "foo",
		tmplText: `{{ define "foo" }}{{ range .Alerts.Firing }}{{ end }}{{ end }}{{ define "bar" }}{{ range .Alerts.Resolved }}{{ end }}{{ end }}`,
		expected: []string{"bar", "foo"},
	}, {
		name:     "returns template name for template text with block action",
		tmplName: "foo",
		tmplText: `{{ block "bar" . }}{{ range .Alerts }}{{ end }}{{ end }}`,
		expected: []string{"foo"},
	}, {
		name:     "returns template name for template text with block action that calls another template",
		tmplName: "foo",
		tmplText: `{{ template "bar" }}{{ range .Alerts }}{{ end }}{{ block "baz" . }}{{ template "bar" . }}{{ end }}`,
		expected: []string{"foo"},
	}, {
		name:     "returns template name for template text that calls another template",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range .Alerts }}{{ end }}{{ end }}{{ template "bar" . }}`,
		expected: []string{"foo"},
	}, {
		name:     "returns template name for template text in if action",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range . }}The labels are {{.Labels }}{{ end }}{{ end }}{{ if len .Alerts }}{{ template "bar" .Alerts }}{{ end }}`,
		expected: []string{"foo"},
	}, {
		name:     "returns template name for template text in range action",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}The labels are {{.Labels }}{{ end }}{{ range .Alerts }}{{ template "bar" . }}{{ end }}`,
		expected: []string{"foo"},
	}, {
		name:     "returns template name for template text in 'with' action",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range . }}The labels are {{.Labels }}{{ end }}{{ end }}{{ with .Alerts }}{{ template "bar" . }}{{ end }}`,
		expected: []string{"foo"},
	}, {
		name:     "returns name of nested templates that each call another nested template",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range .Alerts }}{{ end }}{{ end }}{{ define "qux" }}{{ range .Alerts }}{{ end }}{{ end }}{{ define "baz" }}{{ template "qux" . }}{{ end }}`,
		expected: []string{"bar", "baz"},
	}, {
		name:     "returns nested template for template text that calls another nested template in if action",
		tmplName: "foo",
		tmplText: `{{ define "baz" }}{{ range . }}The labels are {{.Labels }}{{ end }}{{ end }}{{ define "bar" }}{{ if len .Alerts }}{{ template "baz" .Alerts }}{{ end }}{{ end }}`,
		expected: []string{"bar"},
	}, {
		name:     "returns nested template for template text that calls another nested template in 'with' action",
		tmplName: "foo",
		tmplText: `{{ define "baz" }}The labels are {{.Labels }}{{ end }}{{ define "bar" }}{{ with .Alerts }}{{ template "baz" . }}{{ end }}{{ end }}`,
		expected: []string{"bar"},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tmpl, err := template.New(test.tmplName).Parse(test.tmplText)
			require.NoError(t, err)
			actual, err := topTemplates(tmpl)
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}
