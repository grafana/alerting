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
		name:     "inline template",
		tmplName: "foo",
		tmplText: `{{ range .Alerts }}{{ end }}`,
		expected: []string{"foo"},
	}, {
		name:     "nested template",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range .Alerts }}{{ end }}{{ end }}`,
		expected: []string{"bar"},
	}, {
		name:     "inline call to nested template",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range .Alerts }}{{ end }}{{ end }}{{ template "bar" . }}`,
		expected: []string{"foo"},
	}, {
		name:     "inline call to nested template in if",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range . }}The labels are {{.Labels }}{{ end }}{{ end }}{{ if len .Alerts }}{{ template "bar" .Alerts }}{{ end }}`,
		expected: []string{"foo"},
	}, {
		name:     "inline call to nested template in range",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}The labels are {{.Labels }}{{ end }}{{ range .Alerts }}{{ template "bar" . }}{{ end }}`,
		expected: []string{"foo"},
	}, {
		name:     "inline call to nested template in with",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range . }}The labels are {{.Labels }}{{ end }}{{ end }}{{ with .Alerts }}{{ template "bar" . }}{{ end }}`,
		expected: []string{"foo"},
	}, {
		name:     "inline call to nested template that calls other nested template in if",
		tmplName: "foo",
		tmplText: `{{ define "baz" }}{{ range . }}The labels are {{.Labels }}{{ end }}{{ end }}{{ define "bar" }}{{ if len .Alerts }}{{ template "baz" .Alerts }}{{ end }}{{ end }}`,
		expected: []string{"bar"},
	}, {
		name:     "inline call to nested template that calls other nested template in range",
		tmplName: "foo",
		tmplText: `{{ define "baz" }}The labels are {{.Labels }}{{ end }}{{ define "bar" }}{{ with .Alerts }}{{ template "baz" . }}{{ end }}{{ end }}`,
		expected: []string{"bar"},
	}, {
		name:     "multiple top-level templates",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range .Alerts }}{{ end }}{{ end }}{{ define "baz" }}{{ range .Alerts }}{{ end }}{{ end }}`,
		expected: []string{"bar", "baz"},
	}, {
		name:     "multiple top-level templates with inner",
		tmplName: "foo",
		tmplText: `{{ define "bar" }}{{ range .Alerts }}{{ end }}{{ end }}{{ define "inner" }}{{ range .Alerts }}{{ end }}{{ end }}{{ define "baz" }}{{template "inner" . }}{{ end }}`,
		expected: []string{"bar", "baz"},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tmpl, err := template.New(test.tmplName).Parse(test.tmplText)
			require.NoError(t, err)
			actual, err := FindTopLevelTemplates(tmpl)
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}
