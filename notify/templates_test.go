package notify

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/alerting/templates"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	amv2 "github.com/prometheus/alertmanager/api/v2/models"
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
				Name:  "slack.title",
				Text:  "Template Contents",
				Scope: rootScope,
			}},
			Errors: nil,
		},
	}, {
		name: "valid template with builtin function",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ "Template Contents" | len }}{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name:  "slack.title",
				Text:  "17",
				Scope: rootScope,
			}},
			Errors: nil,
		},
	}, {
		name: "valid template with default function",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ "Template Contents" | toUpper }}{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name:  "slack.title",
				Text:  "TEMPLATE CONTENTS",
				Scope: rootScope,
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
				Error: "template: slack.title:1: unclosed action",
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
				Name:  "slack.title",
				Kind:  ExecutionError,
				Error: `template: :1:38: executing "slack.title" at <{{template "missing" .}}>: template "missing" not defined`,
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
				Name:  "slack.title",
				Text:  "Other Contents",
				Scope: rootScope,
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
				Name:  "discord.title",
				Text:  "Discord Title",
				Scope: rootScope,
			}, {
				Name:  "slack.title",
				Text:  "Other Contents",
				Scope: rootScope,
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
				Name:  "discord.title",
				Text:  "Discord Title",
				Scope: rootScope,
			}},
			Errors: []TestTemplatesErrorResult{{
				Name:  "slack.title",
				Kind:  ExecutionError,
				Error: `template: :1:91: executing "other" at <{{template "missing" .}}>: template "missing" not defined`,
			}},
		},
	}, {
		name: "gomplate template",
		input: TestTemplatesConfigBodyParams{
			Alerts: []*amv2.PostableAlert{&simpleAlert},
			Name:   "slack.title",
			Template: `{{ define "now" }}{{ time.Now.Year }}{{ end }}
{{ define "dict" }}{{ coll.Dict "testkey" "testval" | data.ToJSON }}{{ end }}
{{ define "dict.pretty" }}{{ coll.Dict "testkey" "testval" | data.ToJSONPretty " "}}{{ end }}
{{ define "slice" }}{{ coll.Slice "testkey" "testval" | coll.Append "appended" | data.ToJSON}}{{ end }}
{{ define "tmpl" }}{{ coll.Slice "testkey" (tmpl.Exec "slice" . | data.JSON) | coll.Append (tmpl.Inline "{{print .Receiver}}" .) | data.ToJSON}}{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name:  "dict",
				Text:  `{"testkey":"testval"}`,
				Scope: rootScope,
			}, {
				Name:  "dict.pretty",
				Text:  "{\n \"testkey\": \"testval\"\n}",
				Scope: rootScope,
			}, {
				Name:  "now",
				Text:  fmt.Sprint(time.Now().Year()),
				Scope: rootScope,
			}, {
				Name:  "slice",
				Text:  `["testkey","testval","appended"]`,
				Scope: rootScope,
			}, {
				Name:  "tmpl",
				Text:  `["testkey",["testkey","testval","appended"],"TestReceiver"]`,
				Scope: rootScope,
			}},
			Errors: nil,
		},
	}, {
		name: "gomplate data.JSON eJSON not supported",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "ejson",
			Template: `{{ define "ejson" }}{{ coll.Dict "_public_key" "someval" "otherkey" "otherval" | data.ToJSON }}{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name: "ejson",
				// Defense against unintentionally adding eJSON support by importing gomplate data.JSON.
				// Gomplate extracts the _public_key field and attempts to access the ENV.
				Text:  `{"_public_key":"someval","otherkey":"otherval"}`,
				Scope: rootScope,
			}},
			Errors: nil,
		},
	}, {
		name: "gomplate env.Getenv not available",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ env.Getenv "HOME" }}{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: nil,
			Errors: []TestTemplatesErrorResult{{
				Kind:  InvalidTemplate,
				Error: `template: slack.title:1: function "env" not defined`,
			}},
		},
	}, {
		name: "gomplate env.ExpandEnv not available",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ env.ExpandEnv "Your path is set to $PATH" }}{{ end }}`,
		},
		expected: TestTemplatesResults{
			Results: nil,
			Errors: []TestTemplatesErrorResult{{
				Kind:  InvalidTemplate,
				Error: `template: slack.title:1: function "env" not defined`,
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
				Name:  "slack.title",
				Text:  "Template Contents",
				Scope: rootScope,
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
				Name:  "discord.title",
				Text:  "Template Contents",
				Scope: rootScope,
			}},
			Errors: nil,
		},
	}, {
		name: "multiple definitions if wrapper contains non-definition node and name is a defined template",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}Template Contents{{ end }}abcdef`,
		},
		expected: TestTemplatesResults{
			Results: nil,
			Errors: []TestTemplatesErrorResult{{
				Kind:  InvalidTemplate,
				Error: `template: slack.title:1: template: multiple definition of template "slack.title"`,
			}},
		},
	}, {
		name: "empty name and template references itself",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "",
			Template: `{{ define "slack.title" }}Template Contents{{ end }}{{ template "slack.title" . }}`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name:  "",
				Text:  "Template Contents",
				Scope: rootScope,
			}},
			Errors: nil,
		},
	}, {
		name: "empty name and extra non-definition node",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "",
			Template: `{{ define "slack.title" }}Template Contents{{ end }}abcdef`,
		},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name:  "",
				Text:  "abcdef",
				Scope: rootScope,
			}, {
				Name:  "slack.title",
				Text:  "Template Contents",
				Scope: rootScope,
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

	tests := []struct {
		name              string
		existingTemplates []templates.TemplateDefinition
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
				Name:  "slack.title",
				Text:  "[FIRING:1] group_label_value (alert1 val1)",
				Scope: rootScope,
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
		existingTemplates: []templates.TemplateDefinition{{
			Name:     "existing",
			Template: `{{ define "existing" }}Some existing template{{ end }}`,
		}},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name:  "slack.title",
				Text:  "Some existing template",
				Scope: rootScope,
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
		existingTemplates: []templates.TemplateDefinition{{
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}Some existing template{{ end }}`,
		}},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name:  "slack.title",
				Text:  "New template",
				Scope: rootScope,
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
		existingTemplates: []templates.TemplateDefinition{{
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}Some existing template{{ end }}{{ define "slack.alternate_title" }}Some existing alternate template{{ end }}`,
		}},
		expected: TestTemplatesResults{
			Results: nil,
			Errors: []TestTemplatesErrorResult{{
				Name:  "slack.title",
				Kind:  ExecutionError,
				Error: `template: :1:38: executing "slack.title" at <{{template "slack.alternate_title" .}}>: template "slack.alternate_title" not defined`,
			}},
		},
	}, {
		name: "reference a template that was overridden and still defined",
		input: TestTemplatesConfigBodyParams{
			Alerts:   []*amv2.PostableAlert{&simpleAlert},
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}{{ template "slack.alternate_title" . }}{{ end }}{{ define "slack.alternate_title" }}Some new alternate template{{ end }}`,
		},
		existingTemplates: []templates.TemplateDefinition{{
			Name:     "slack.title",
			Template: `{{ define "slack.title" }}Some existing template{{ end }}{{ define "slack.alternate_title" }}Some existing alternate template{{ end }}`,
		}},
		expected: TestTemplatesResults{
			Results: []TestTemplatesResult{{
				Name:  "slack.title",
				Text:  "Some new alternate template",
				Scope: rootScope,
			}},
			Errors: nil,
		},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if len(test.existingTemplates) > 0 {
				am.templates = test.existingTemplates
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
	}{
		{
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
GroupLabels: {{ range .GroupLabels.SortedPairs }}{{ .Name }}={{ .Value }}{{ end }}
CommonLabels: {{ range .CommonLabels.SortedPairs }}{{ .Name }}={{ .Value }}{{ end }}
CommonAnnotations: {{ range .CommonAnnotations.SortedPairs }}{{ .Name }}={{ .Value }}{{ end }}
{{ end }}`,
			},
			expected: TestTemplatesResults{
				Results: []TestTemplatesResult{{
					Name:  "slack.title",
					Text:  "\nReceiver: TestReceiver\nStatus: firing\nExternalURL: http://localhost:9093\nAlerts: 1\nFiring Alerts: 1\nResolved Alerts: 0\nGroupLabels: group_label=group_label_value\nCommonLabels: alertname=alert1lbl1=val1\nCommonAnnotations: ann1=annv1\n",
					Scope: rootScope,
				}},
				Errors: nil,
			},
		},
		{
			name: "template scoped to .Alerts",
			input: TestTemplatesConfigBodyParams{
				Alerts: []*amv2.PostableAlert{&simpleAlert},
				Name:   "alerts.custom",
				Template: `{{ define "alerts.custom" }}{{ range . }}
Labels:
{{ range .Labels.SortedPairs }} - {{ .Name }} = {{ .Value }}
{{ end }}Annotations:
{{ range .Annotations.SortedPairs }} - {{ .Name }} = {{ .Value }}
{{ end }}{{ if gt (len .GeneratorURL) 0 }}Source: {{ .GeneratorURL }}
{{ end }}{{ if gt (len .SilenceURL) 0 }}Silence: {{ .SilenceURL }}
{{ end }}{{ if gt (len .DashboardURL) 0 }}Dashboard: {{ .DashboardURL }}
{{ end }}{{ if gt (len .PanelURL) 0 }}Panel: {{ .PanelURL }}
{{ end }}{{ end }}{{ end }}`,
			},
			expected: TestTemplatesResults{
				Results: []TestTemplatesResult{{
					Name:  "alerts.custom",
					Text:  "\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost:9093/alerting/silence/new?alertmanager=grafana&matcher=__alert_rule_uid__%3Drule+uid&matcher=lbl1%3Dval1\nDashboard: http://localhost:9093/d/abcd\nPanel: http://localhost:9093/d/abcd?viewPanel=efgh\n",
					Scope: alertsScope,
				}},
				Errors: nil,
			},
		},
		{
			name: "template scoped to .Alert",
			input: TestTemplatesConfigBodyParams{
				Alerts: []*amv2.PostableAlert{&simpleAlert},
				Name:   "alerts.custom.single",
				Template: `{{ define "alerts.custom.single" }}
Labels:
{{ range .Labels.SortedPairs }} - {{ .Name }} = {{ .Value }}
{{ end }}Annotations:
{{ range .Annotations.SortedPairs }} - {{ .Name }} = {{ .Value }}
{{ end }}{{ if gt (len .GeneratorURL) 0 }}Source: {{ .GeneratorURL }}
{{ end }}{{ if gt (len .SilenceURL) 0 }}Silence: {{ .SilenceURL }}
{{ end }}{{ if gt (len .DashboardURL) 0 }}Dashboard: {{ .DashboardURL }}
{{ end }}{{ if gt (len .PanelURL) 0 }}Panel: {{ .PanelURL }}
{{ end }}{{ end }}`,
			},
			expected: TestTemplatesResults{
				Results: []TestTemplatesResult{{
					Name:  "alerts.custom.single",
					Text:  "\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost:9093/alerting/silence/new?alertmanager=grafana&matcher=__alert_rule_uid__%3Drule+uid&matcher=lbl1%3Dval1\nDashboard: http://localhost:9093/d/abcd\nPanel: http://localhost:9093/d/abcd?viewPanel=efgh\n",
					Scope: alertScope,
				}},
				Errors: nil,
			},
		},
		{
			name: "failing scope",
			input: TestTemplatesConfigBodyParams{
				Alerts:   []*amv2.PostableAlert{&simpleAlert},
				Name:     "alerts.custom.failing",
				Template: `{{ define "alerts.custom.failing" }}{{ .DOESNOTEXIST }} {{ end }}`,
			},
			expected: TestTemplatesResults{
				Results: nil,
				Errors: []TestTemplatesErrorResult{{
					Name:  "alerts.custom.failing",
					Kind:  ExecutionError,
					Error: `template: :1:39: executing "alerts.custom.failing" at <.DOESNOTEXIST>: can't evaluate field DOESNOTEXIST in type *templates.ExtendedData`,
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
