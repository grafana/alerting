package receivers

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	gomail "gopkg.in/mail.v2"

	"github.com/stretchr/testify/require"
)

func TestEmbedTemplate(t *testing.T) {
	// Test the email templates are embedded and parsed correctly.
	require.NotEmpty(t, defaultEmailTemplate)
	s, err := NewEmailSender(EmailSenderConfig{})
	require.NoError(t, err)

	ds, ok := s.(*defaultEmailSender)
	require.True(t, ok)

	definedTmpls := ds.tmpl.DefinedTemplates()
	require.Contains(t, definedTmpls, "\"ng_alert_notification.html\"")
	require.Contains(t, definedTmpls, "\"ng_alert_notification.txt\"")
}

func TestBuildEmailMessage(t *testing.T) {
	testValue := "test-value"
	testData := map[string]interface{}{"Value": testValue}
	externalURL := "http://test.org"
	sentBy := "Grafana testVersion"

	tests := []struct {
		name             string
		contentTypes     []string
		data             map[string]interface{}
		subject          string
		template         string
		templateName     string
		embeddedFiles    []string
		embeddedContents []EmbeddedContent
		expErr           string
		expSubject       string
		expBody          string
	}{
		{
			name:         "no subject",
			template:     fmt.Sprintf("{{ define %q -}} test {{- end }}", "test_template"),
			templateName: "test_template",
			expErr:       "missing subject in template test_template",
		},
		{
			name:          "subject in template, template data provided",
			contentTypes:  []string{"text/plain"},
			data:          testData,
			template:      fmt.Sprintf("{{ define %q -}} {{ Subject .Subject .TemplateData %q }} {{ .AppUrl }} {{ .SentBy }} {{- end }}", "test_template.txt", "{{ .Value }}"),
			templateName:  "test_template",
			embeddedFiles: []string{"embedded-1", "embedded-2"},
			embeddedContents: []EmbeddedContent{
				{Name: "embedded-1", Content: []byte("embedded-1 data")},
				{Name: "embedded-2", Content: []byte("embedded-2 data")},
			},
			expSubject: testValue,
			expBody:    fmt.Sprintf("%s %s %s", testValue, externalURL, sentBy),
		},
		{
			name:         "subject via config, template data provided",
			contentTypes: []string{"text/html"},
			data:         testData,
			subject:      "test_subject",
			template:     fmt.Sprintf("{{ define %q -}} {{ .TemplateData.Value }} {{ .AppUrl }} {{ .SentBy }} {{- end }}", "test_template.html"),
			templateName: "test_template",
			expSubject:   "test_subject",
			expBody:      fmt.Sprintf("%s %s %s", testValue, externalURL, sentBy),
		},
		{
			name:         "default data only",
			contentTypes: []string{"text/plain"},
			subject:      "test_subject",
			template:     fmt.Sprintf("{{ define %q -}} {{ .TemplateData.Value }} {{ .AppUrl }} {{ .SentBy }} {{- end }}", "test_template.txt"),
			templateName: "test_template",
			expSubject:   "test_subject",
			expBody:      fmt.Sprintf(" %s %s", externalURL, sentBy),
		},
		{
			name:         "attempting to execute an undefined template",
			contentTypes: []string{"text/html", "text/plain"},
			subject:      "test_subject",
			templateName: "undefined",
			expErr:       `html/template: "undefined.html" is undefined`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, err := NewEmailSender(EmailSenderConfig{
				ContentTypes: test.contentTypes,
				ExternalURL:  externalURL,
				SentBy:       sentBy,
			})
			require.NoError(t, err)
			ds, ok := s.(*defaultEmailSender)
			require.True(t, ok)

			_, err = ds.tmpl.Parse(test.template)
			require.NoError(t, err)

			cfg := SendEmailSettings{
				To:               []string{"test@test.com"},
				SingleEmail:      true,
				Template:         test.templateName,
				Data:             test.data,
				ReplyTo:          []string{"test2@test.com"},
				EmbeddedFiles:    test.embeddedFiles,
				EmbeddedContents: test.embeddedContents,
				Subject:          test.subject,
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
				require.Equal(t, cfg.EmbeddedContents, m.EmbeddedContents)
				require.Equal(t, test.expSubject, m.Subject)

				for _, ct := range test.contentTypes {
					require.Equal(t, test.expBody, m.Body[ct])
				}
			}
		})
	}
}

func TestCreateDialer(t *testing.T) {
	tests := []struct {
		name   string
		cfg    EmailSenderConfig
		expErr string
	}{
		{
			"invalid host",
			EmailSenderConfig{
				Host: "http://localhost:1234",
			},
			"address http://localhost:1234: too many colons in address",
		},
		{
			"port is not an integer",
			EmailSenderConfig{
				Host: "localhost:abc",
			},
			"strconv.Atoi: parsing \"abc\": invalid syntax",
		},
		{
			"non-existent cert file",
			EmailSenderConfig{
				Host:     "localhost:1234",
				CertFile: "non-existent.pem",
			},
			"could not load cert or key file: open non-existent.pem: no such file or directory",
		},
		{
			"success case",
			EmailSenderConfig{
				SkipVerify:     true,
				Host:           "localhost:1234",
				StartTLSPolicy: "MandatoryStartTLS",
			},
			"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, err := NewEmailSender(test.cfg)
			require.NoError(t, err)
			ds, ok := s.(*defaultEmailSender)
			require.True(t, ok)

			d, err := ds.createDialer()
			if test.expErr != "" {
				require.EqualError(t, err, test.expErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.cfg.EhloIdentity, d.LocalName)
				require.Equal(t, test.cfg.SkipVerify, d.TLSConfig.InsecureSkipVerify)
				require.Equal(t, strings.Split(test.cfg.Host, ":")[0], d.TLSConfig.ServerName)
				require.Equal(t, getStartTLSPolicy(test.cfg.StartTLSPolicy), d.StartTLSPolicy)
			}
		})
	}
}

func TestBuildEmail(t *testing.T) {
	cfg := EmailSenderConfig{
		ContentTypes: []string{"text/html", "text/plain"},
		StaticHeaders: map[string]string{
			"Header-1": "value-1",
			"Header-2": "value-2",
		},
	}

	s, err := NewEmailSender(cfg)
	require.NoError(t, err)
	ds, ok := s.(*defaultEmailSender)
	require.True(t, ok)

	mCfg := Message{
		From:    "test@test.com",
		To:      []string{"to1@to.com", "to2@to.com"},
		Subject: "Test Subject",
		ReplyTo: []string{"reply1@reply.com", "reply2@reply.com"},
		Body:    map[string]string{"text/plain": "This is a test message"},
	}
	m := ds.buildEmail(&mCfg)
	require.Equal(t, []string{mCfg.From}, m.GetHeader("From"))
	require.Equal(t, mCfg.To, m.GetHeader("To"))
	require.Equal(t, []string{mCfg.Subject}, m.GetHeader("Subject"))
	require.Equal(t, []string{strings.Join(mCfg.ReplyTo, ", ")}, m.GetHeader("Reply-To"))
	for k, v := range cfg.StaticHeaders {
		require.Equal(t, []string{v}, m.GetHeader(k))
	}

	var buf bytes.Buffer
	_, err = m.WriteTo(&buf)
	require.NoError(t, err)

	str := buf.String()
	require.Contains(t, str, mCfg.Body["text/plain"])
	require.Contains(t, str, mCfg.Body["text/html"])
}

type mockSender struct {
	sentTo [][]string
	err    error
}

func (m *mockSender) Send(_ string, to []string, _ io.WriterTo) error {
	m.sentTo = append(m.sentTo, to)
	return m.err
}
func (m *mockSender) Close() error { return nil }

func TestSend(t *testing.T) {
	tests := []struct {
		name         string
		message      *Message
		expectedSent [][]string
		senderError  error
		expectedErr  bool
	}{
		{
			name:    "SingleEmail=true",
			message: makeMsg(true, "a@b.com", "c@d.com"),
			expectedSent: [][]string{
				{"a@b.com", "c@d.com"},
			},
			expectedErr: false,
		},
		{
			name:    "SingleEmail=false",
			message: makeMsg(false, "a@b.com", "c@d.com", "e@f.com"),
			expectedSent: [][]string{
				{"a@b.com"},
				{"c@d.com"},
				{"e@f.com"},
			},
			expectedErr: false,
		},
		{
			name:         "Send error with SingleEmail=true",
			message:      makeMsg(true, "a@b.com", "c@d.com"),
			senderError:  fmt.Errorf("send error"),
			expectedSent: [][]string{{"a@b.com", "c@d.com"}},
			expectedErr:  true,
		},
		{
			name:         "Send error with SingleEmail=false",
			message:      makeMsg(false, "a@b.com", "c@d.com"),
			senderError:  fmt.Errorf("send error"),
			expectedSent: [][]string{{"a@b.com"}, {"c@d.com"}},
			expectedErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := &mockSender{err: tc.senderError}

			ds := &defaultEmailSender{
				cfg:  EmailSenderConfig{},
				tmpl: nil,
				dialFn: func(_ *defaultEmailSender) (gomail.SendCloser, error) {
					return ms, nil
				},
			}

			sentCount, err := ds.Send(tc.message)

			if tc.senderError != nil {
				require.Equal(t, 0, sentCount)
				require.Error(t, err)
				require.Contains(t, err.Error(), strings.Join(tc.message.To, ";"))
			} else {
				require.Equal(t, len(tc.expectedSent), sentCount)
				require.NoError(t, err)
			}

			require.ElementsMatch(t, tc.expectedSent, ms.sentTo)
		})
	}
}

func makeMsg(single bool, addrs ...string) *Message {
	return &Message{
		SingleEmail: single,
		To:          addrs,
		From:        "noreply@grafana.com",
		Subject:     "Subject",
		Body:        map[string]string{"text/plain": "body"},
	}
}
