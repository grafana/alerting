package email

import (
	"bytes"
	"testing"

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
