package notify

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/go-openapi/strfmt"
	amv2 "github.com/prometheus/alertmanager/api/v2/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	simpleAlert = amv2.PostableAlert{
		Alert: amv2.Alert{
			Labels: amv2.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
		},
		Annotations: amv2.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
		StartsAt:    strfmt.DateTime{},
		EndsAt:      strfmt.DateTime{},
	}
)

func TestTemplateSimple(t *testing.T) {
	am, _ := setupAMTest(t)

	tests := []struct {
		name     string
		input    TestTemplatesConfigBodyParams
		expected TestTemplatesResults
	}{{
		name: "valid template",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}Template Contents{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "slack.title",
				Text: "Template Contents",
			}},
			Errors: nil,
		},
	}, {
		name: "invalid template",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}Template Contents{{ end `,
		},
		expected: TestTemplatesResults{
			Results: nil,
			Errors: []TestTemplatesErrorResult{{
				Kind:  InvalidTemplate,
				Error: errors.New("template: slack.title:1: unclosed action"),
			}},
		},
	}, {
		name: "execution error on missing template",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ template "missing" . }}{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: nil,
			Errors: []TestTemplatesErrorResult{{
				Name: "slack.title",
				Kind: ExecutionError,
				Error: template.ExecError{
					Name: "slack.title",
					Err:  errors.New(`template: slack.title:1:38: executing "slack.title" at <{{template "missing" .}}>: template "missing" not defined`),
				},
			}},
		},
	}, {
		name: "valid template referencing other template inside test",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ template "other" . }}{{ end }}{{ define "other" }}Other Contents{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "slack.title",
				Text: "Other Contents",
			}},
			Errors: nil,
		},
	}, {
		name: "valid template only return top-levels",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ template "other" . }}{{ end }}{{ define "other" }}Other Contents{{ end }}{{ define "discord.title" }}Discord Title{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "discord.title",
				Text: "Discord Title",
			}, {
				Name: "slack.title",
				Text: "Other Contents",
			}},
			Errors: nil,
		},
	}, {
		name: "mixed templates some execution errors",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ template "other" . }}{{ end }}{{ define "other" }}{{ template "missing" . }}{{ end }}{{ define "discord.title" }}Discord Title{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "discord.title",
				Text: "Discord Title",
			}},
			Errors: []TestTemplatesErrorResult{{
				Name: "slack.title",
				Kind: ExecutionError,
				Error: template.ExecError{
					Name: "other",
					Err:  errors.New(`template: slack.title:1:91: executing "other" at <{{template "missing" .}}>: template "missing" not defined`),
				},
			}},
		},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := am.TestTemplate(context.Background(), test.input)
			require.NoError(t, err)
			assert.Equal(t, test.expected, *res)
		})
	}
}

func TestTemplateSpecialCases(t *testing.T) {
	am, _ := setupAMTest(t)

	tests := []struct {
		name     string
		input    TestTemplatesConfigBodyParams
		expected TestTemplatesResults
	}{{
		name: "template with no name",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "",
			Template: `{{ define "slack.title" }}Template Contents{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "slack.title",
				Text: "Template Contents",
			}},
			Errors: nil,
		},
	}, {
		name: "empty template",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: ``,
		},
		expected: TestTemplatesResults{
			Results: nil,
			Errors:  nil,
		},
	}, {
		name: "name is not a defined template",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "discord.title" }}Template Contents{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "discord.title",
				Text: "Template Contents",
			}},
			Errors: nil,
		},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := am.TestTemplate(context.Background(), test.input)
			require.NoError(t, err)
			assert.Equal(t, test.expected, *res)
		})
	}
}

func TestTemplateWithExistingTemplates(t *testing.T) {
	am, _ := setupAMTest(t)
	tmpDir, err := os.MkdirTemp("", "test-templates")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(tmpDir))
	})

	am.workingDirectory = tmpDir

	tests := []struct {
		name              string
		existingTemplates map[string]string
		input             TestTemplatesConfigBodyParams
		expected          TestTemplatesResults
	}{{
		name: "valid template referencing a default template",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ template "slack.default.title" . }}{{ end }}`,
		},
		existingTemplates: nil,
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "slack.title",
				Text: "[FIRING:1] group_label_value (alert1 val1)",
			}},
			Errors: nil,
		},
	}, {
		name: "valid template referencing an existing template",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ template "existing" . }}{{ end }}`,
		},
		existingTemplates: map[string]string{
			"existing": `{{ define "existing" }}Some existing template{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "slack.title",
				Text: "Some existing template",
			}},
			Errors: nil,
		},
	}, {
		name: "valid template overriding an existing template",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}New template{{ end }}`,
		},
		existingTemplates: map[string]string{
			"slack.title": `{{ define "slack.title" }}Some existing template{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "slack.title",
				Text: "New template",
			}},
			Errors: nil,
		},
	}, {
		name: "reference a template that was overridden and no longer defined",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ template "slack.alternate_title" . }}{{ end }}`,
		},
		existingTemplates: map[string]string{
			"slack.title": `{{ define "slack.title" }}Some existing template{{ end }}{{ define "slack.alternate_title" }}Some existing alternate template{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: nil,
			Errors: []TestTemplatesErrorResult{{
				Name: "slack.title",
				Kind: ExecutionError,
				Error: template.ExecError{
					Name: "slack.title",
					Err:  errors.New(`template: slack.title:1:38: executing "slack.title" at <{{template "slack.alternate_title" .}}>: template "slack.alternate_title" not defined`),
				},
			}},
		},
	}, {
		name: "reference a template that was overridden and still defined",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ template "slack.alternate_title" . }}{{ end }}{{ define "slack.alternate_title" }}Some new alternate template{{ end }}`,
		},
		existingTemplates: map[string]string{
			"slack.title": `{{ define "slack.title" }}Some existing template{{ end }}{{ define "slack.alternate_title" }}Some existing alternate template{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "slack.title",
				Text: "Some new alternate template",
			}},
			Errors: nil,
		},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if len(test.existingTemplates) > 0 {
				for name, tmpl := range test.existingTemplates {
					createTemplate(t, tmpDir, name, tmpl)
					am.templates = append(am.templates, name)
				}
			}
			res, err := am.TestTemplate(context.Background(), test.input)
			require.NoError(t, err)
			assert.Equal(t, test.expected, *res)
		})
	}
}

func TestTemplateAlertData(t *testing.T) {
	am, _ := setupAMTest(t)
	am.externalURL = "http://localhost:9093"

	tests := []struct {
		name     string
		input    TestTemplatesConfigBodyParams
		expected TestTemplatesResults
	}{{
		name: "check various extended data",
		input: TestTemplatesConfigBodyParams{
			Alerts: []*amv2.PostableAlert{&simpleAlert},
			Name:   "slack.title",
			Template: `{{ define "slack.title" }}
Receiver: {{ .Receiver }}
Status: {{ .Status }}
ExternalURL: {{ .ExternalURL }}
Alerts: {{ len .Alerts }}
Firing Alerts: {{ len .Alerts.Firing }}
Resolved Alerts: {{ len .Alerts.Resolved }}
GroupLabels: {{ .GroupLabels }}
CommonLabels: {{ .CommonLabels }}
CommonAnnotations: {{ .CommonAnnotations }}
{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "slack.title",
				Text: "\nReceiver: TestReceiver\nStatus: firing\nExternalURL: http://localhost:9093\nAlerts: 1\nFiring Alerts: 1\nResolved Alerts: 0\nGroupLabels: map[group_label:group_label_value]\nCommonLabels: map[alertname:alert1 lbl1:val1]\nCommonAnnotations: map[ann1:annv1]\n",
			}},
			Errors: nil,
		},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := am.TestTemplate(context.Background(), test.input)
			require.NoError(t, err)
			assert.Equal(t, test.expected, *res)
		})
	}
}

func createTemplate(t *testing.T, tmpDir string, name string, tmpl string) {
	f, err := os.Create(filepath.Join(tmpDir, name))
	require.NoError(t, err)
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(f.Name()))
	})

	_, err = f.WriteString(tmpl)
	require.NoError(t, err)
}
