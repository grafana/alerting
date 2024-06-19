package receivers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbedTemplate(t *testing.T) {
	// Test the email template is embedded and parsed correctly.
	require.NotEmpty(t, defaultEmailTemplate)

	_, err := NewEmailSenderFactory(EmailSenderConfig{})(Metadata{})
	require.NoError(t, err)
}

func TestBuildEmailMessage(t *testing.T) {
	testValue := "test-value"
	testData := map[string]interface{}{"Value": testValue}
	externalURL := "http://test.org"
	buildVersion := "testVersion"

	tests := []struct {
		name         string
		data         map[string]interface{}
		subject      string
		template     string
		templateName string
		expErr       string
		expSubject   string
		expBody      string
	}{
		{
			name:         "no subject",
			template:     fmt.Sprintf("{{ define %q -}} test {{- end }}", "test_template"),
			templateName: "test_template",
			expErr:       "missing subject in template test_template",
		},
		{
			name:         "subject in template, template data provided",
			data:         testData,
			template:     fmt.Sprintf("{{ define %q -}} {{ Subject .Subject .TemplateData %q }} {{ .AppUrl }} {{ .BuildVersion }} {{- end }}", "test_template", "{{ .Value }}"),
			templateName: "test_template",
			expSubject:   testValue,
			expBody:      fmt.Sprintf("%s %s %s", testValue, externalURL, buildVersion),
		},
		{
			name:         "subject via config, template data provided",
			data:         testData,
			subject:      "test_subject",
			template:     fmt.Sprintf("{{ define %q -}} {{ .TemplateData.Value }} {{ .AppUrl }} {{ .BuildVersion }} {{- end }}", "test_template"),
			templateName: "test_template",
			expSubject:   "test_subject",
			expBody:      fmt.Sprintf("%s %s %s", testValue, externalURL, buildVersion),
		},
		{
			name:         "default data only",
			subject:      "test_subject",
			template:     fmt.Sprintf("{{ define %q -}} {{ .TemplateData.Value }} {{ .AppUrl }} {{ .BuildVersion }} {{- end }}", "test_template"),
			templateName: "test_template",
			expSubject:   "test_subject",
			expBody:      fmt.Sprintf(" %s %s", externalURL, buildVersion),
		},
		{
			name:         "attempting to execute an undefined template",
			templateName: "undefined",
			expErr:       `html/template: "undefined" is undefined`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, err := NewEmailSenderFactory(EmailSenderConfig{
				ExternalURL: externalURL,
				Version:     buildVersion,
			})(Metadata{})
			require.NoError(t, err)
			ds, ok := s.(*defaultEmailSender)
			require.True(t, ok)

			_, err = ds.tmpl.Parse(test.template)
			require.NoError(t, err)

			cfg := SendEmailSettings{
				To:            []string{"test@test.com"},
				SingleEmail:   true,
				Template:      test.templateName,
				Data:          test.data,
				ReplyTo:       []string{"test2@test.com"},
				EmbeddedFiles: []string{},
				AttachedFiles: []*SendEmailAttachedFile{},
				Subject:       test.subject,
			}
			m, err := ds.buildEmailMessage(&cfg)
			if test.expErr != "" {
				require.EqualError(t, err, test.expErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, cfg.To, m.To)
				require.Equal(t, cfg.SingleEmail, m.SingleEmail)
				require.Equal(t, cfg.ReplyTo, m.ReplyTo)
				require.Equal(t, cfg.EmbeddedFiles, m.EmbeddedFiles)
				require.Equal(t, cfg.AttachedFiles, m.AttachedFiles)
				require.Equal(t, test.expSubject, m.Subject)
				require.Equal(t, test.expBody, m.Body)
			}
		})
	}

}
