package receivers

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/grafana/alerting/templates/email"

	gomail "gopkg.in/mail.v2"

	"github.com/stretchr/testify/require"
)

func TestEmailTemplateInitialized(t *testing.T) {
	// Test the email template singleton is initialized.
	tmpl := email.Template()
	require.NotNil(t, tmpl)
}

func TestBuildEmailMessage(t *testing.T) {
	externalURL := "http://test.org"
	sentBy := "Grafana testVersion"

	t.Run("undefined template returns error", func(t *testing.T) {
		s := NewEmailSender(EmailSenderConfig{
			ContentTypes: []string{"text/html", "text/plain"},
			ExternalURL:  externalURL,
			SentBy:       sentBy,
		})
		ds, ok := s.(*defaultEmailSender)
		require.True(t, ok)

		cfg := SendEmailSettings{
			To:          []string{"test@test.com"},
			SingleEmail: true,
			Template:    "undefined",
			Subject:     "test_subject",
		}
		_, err := ds.buildEmailMessage(&cfg)
		require.ErrorContains(t, err, `html/template: "undefined.html" is undefined`)
	})

	t.Run("unsupported content type returns error", func(t *testing.T) {
		s := NewEmailSender(EmailSenderConfig{
			ContentTypes: []string{"application/json"},
			ExternalURL:  externalURL,
			SentBy:       sentBy,
		})
		ds, ok := s.(*defaultEmailSender)
		require.True(t, ok)

		cfg := SendEmailSettings{
			To:          []string{"test@test.com"},
			SingleEmail: true,
			Template:    "ng_alert_notification",
			Subject:     "test_subject",
		}
		_, err := ds.buildEmailMessage(&cfg)
		require.ErrorContains(t, err, `unrecognized content type "application/json"`)
	})
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
			s := NewEmailSender(test.cfg)
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
	tests := []struct {
		name             string
		cfg              EmailSenderConfig
		msg              Message
		expectedTo       []string
		expectedBcc      []string
		checkBodyContent bool
	}{
		{
			name: "Standard email with To header",
			cfg: EmailSenderConfig{
				ContentTypes: []string{"text/html", "text/plain"},
				StaticHeaders: map[string]string{
					"Header-1": "value-1",
					"Header-2": "value-2",
				},
			},
			msg: Message{
				From:    "test@test.com",
				To:      []string{"to1@to.com", "to2@to.com"},
				Subject: "Test Subject",
				ReplyTo: []string{"reply1@reply.com", "reply2@reply.com"},
				Body:    map[string]string{"text/plain": "This is a test message"},
			},
			expectedTo:       []string{"to1@to.com", "to2@to.com"},
			expectedBcc:      nil,
			checkBodyContent: true,
		},
		{
			name: "UseBCC=true, SingleEmail=false - recipients in Bcc",
			cfg: EmailSenderConfig{
				UseBCC:       true,
				ContentTypes: []string{"text/plain"},
			},
			msg: Message{
				From:        "test@test.com",
				To:          []string{"to1@to.com", "to2@to.com"},
				Subject:     "Test Subject",
				SingleEmail: false,
				Body:        map[string]string{"text/plain": "This is a test message"},
			},
			expectedTo:  nil,
			expectedBcc: []string{"to1@to.com", "to2@to.com"},
		},
		{
			name: "UseBCC=true, SingleEmail=true - recipients in To (UseBCC ignored)",
			cfg: EmailSenderConfig{
				UseBCC:       true,
				ContentTypes: []string{"text/plain"},
			},
			msg: Message{
				From:        "test@test.com",
				To:          []string{"to1@to.com", "to2@to.com"},
				Subject:     "Test Subject",
				SingleEmail: true,
				Body:        map[string]string{"text/plain": "This is a test message"},
			},
			expectedTo:  []string{"to1@to.com", "to2@to.com"},
			expectedBcc: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewEmailSender(tc.cfg)
			ds, ok := s.(*defaultEmailSender)
			require.True(t, ok)

			m := ds.buildEmail(&tc.msg)
			require.Equal(t, []string{tc.msg.From}, m.GetHeader("From"))
			require.Equal(t, tc.expectedTo, m.GetHeader("To"))
			require.Equal(t, tc.expectedBcc, m.GetHeader("Bcc"))
			require.Equal(t, []string{tc.msg.Subject}, m.GetHeader("Subject"))

			if len(tc.msg.ReplyTo) > 0 {
				require.Equal(t, []string{strings.Join(tc.msg.ReplyTo, ", ")}, m.GetHeader("Reply-To"))
			}

			for k, v := range tc.cfg.StaticHeaders {
				require.Equal(t, []string{v}, m.GetHeader(k))
			}

			if tc.checkBodyContent {
				var buf bytes.Buffer
				_, err := m.WriteTo(&buf)
				require.NoError(t, err)

				str := buf.String()
				require.Contains(t, str, tc.msg.Body["text/plain"])
				if _, ok := tc.msg.Body["text/html"]; ok {
					require.Contains(t, str, tc.msg.Body["text/html"])
				}
			}
		})
	}
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
		cfg          EmailSenderConfig
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
		{
			name:    "SingleEmail=false, UseBCC=true",
			message: makeMsg(false, "a@b.com", "c@d.com", "e@f.com"),
			cfg:     EmailSenderConfig{UseBCC: true},
			expectedSent: [][]string{
				{"a@b.com", "c@d.com", "e@f.com"},
			},
			expectedErr: false,
		},
		{
			name:    "SingleEmail=true, UseBCC=true",
			message: makeMsg(true, "a@b.com", "c@d.com"),
			cfg:     EmailSenderConfig{UseBCC: true},
			expectedSent: [][]string{
				{"a@b.com", "c@d.com"},
			},
			expectedErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := &mockSender{err: tc.senderError}

			ds := &defaultEmailSender{
				cfg: tc.cfg,
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
