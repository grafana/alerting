package email

import (
	"bytes"
	"testing"

	"github.com/grafana/alerting/templates"
	"github.com/stretchr/testify/require"
)

func TestTemplateInitialization(t *testing.T) {
	// If init() panicked, this test would never run.
	// This test verifies the singleton is properly initialized.
	tmpl := Template()
	require.NotNil(t, tmpl, "singleton template should be initialized")
}

func TestTemplateContainsExpectedTemplates(t *testing.T) {
	tmpl := Template()

	// Verify expected templates are defined by attempting to execute them
	// with minimal data (we expect errors due to missing data, but not "undefined template" errors)
	var buf bytes.Buffer

	err := tmpl.ExecuteTemplate(&buf, "ng_alert_notification.html", nil)
	require.NotNil(t, err, "expected error due to nil data")
	require.NotContains(t, err.Error(), "is undefined", "ng_alert_notification.html should be defined")

	buf.Reset()
	err = tmpl.ExecuteTemplate(&buf, "ng_alert_notification.txt", nil)
	require.NotNil(t, err, "expected error due to nil data")
	require.NotContains(t, err.Error(), "is undefined", "ng_alert_notification.txt should be defined")
}

func TestTemplateUndefinedTemplateReturnsError(t *testing.T) {
	tmpl := Template()
	var buf bytes.Buffer

	err := tmpl.ExecuteTemplate(&buf, "nonexistent_template", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is undefined")
}

// TestLabelsTableHasFixedLayout guards against regression of the rendering fix
// for the Labels and Annotations key/value tables. table-layout:auto with no
// column widths caused long unbreakable label names to expand the first column
// and crush the second to a hairline in renderer-strict web-based email clients.
func TestLabelsTableHasFixedLayout(t *testing.T) {
	tmpl := Template()

	longLabel := "ExceptionDetail_InnerException_InnerException_InnerException_Message"

	data := map[string]any{
		"Title":   "Test alert",
		"Message": "",
		"Status":  "firing",
		"Alerts": templates.ExtendedAlerts{
			{
				Status:      "firing",
				Labels:      templates.KV{longLabel: "short value"},
				Annotations: templates.KV{"runbook": "https://example.com"},
			},
		},
		"GroupLabels":       templates.KV{},
		"CommonLabels":      templates.KV{},
		"CommonAnnotations": templates.KV{},
		"ExternalURL":       "https://example.com",
		"RuleUrl":           "https://example.com/rule",
		"AlertPageUrl":      "https://example.com/page",
		"Subject":           map[string]any{"executed_template": "Test alert"},
	}

	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, "ng_alert_notification.html", data)
	require.NoError(t, err)

	output := buf.String()

	// Constrain column widths in the Labels / Annotations key/value tables to
	// prevent runaway expansion in renderer-strict clients (New Outlook / OWA).
	require.Contains(t, output, "table-layout:fixed",
		"Labels/Annotations tables must use fixed layout (renderer-strict clients break otherwise)")
	require.Contains(t, output, `width="40%"`,
		"label-name <td> must have explicit width attribute")
	require.Contains(t, output, `width="60%"`,
		"label-value <td> must have explicit width attribute")
	require.Contains(t, output, "word-break:break-word",
		"<td> cells must allow long-string break to avoid horizontal overflow")
	require.NotContains(t, output, "table-layout:auto",
		"no Labels/Annotations table should use table-layout:auto anymore")

	// Sanity check: the long label name should still appear in output
	// (i.e., the fix doesn't silently strip data).
	require.Contains(t, output, longLabel)
}
