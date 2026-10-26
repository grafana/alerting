package receivers

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/alerting/templates/email"

	gomail "gopkg.in/mail.v2"
)

type EmailSenderConfig struct {
	AuthPassword   string
	AuthUser       string
	CertFile       string
	ContentTypes   []string
	EhloIdentity   string
	ExternalURL    string
	FromName       string
	FromAddress    string
	Host           string
	KeyFile        string
	SkipVerify     bool
	StartTLSPolicy string
	StaticHeaders  map[string]string
	SentBy         string
	// UseBCC indicates whether to send emails using BCC
	// instead of the TO field when SingleEmail is false.
	// If both UseBCC and SingleEmail are true, UseBCC is ignored
	// and recipients are placed in the TO field.
	UseBCC bool
	// DialContext, if set, is used to open the connection to the SMTP server.
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)
}

type defaultEmailSender struct {
	cfg    EmailSenderConfig
	dialFn func(*defaultEmailSender) (gomail.SendCloser, error)
}

// NewEmailSender takes a configuration and returns a new EmailSender.
func NewEmailSender(cfg EmailSenderConfig) EmailSender {
	return &defaultEmailSender{
		cfg: cfg,
		dialFn: func(s *defaultEmailSender) (gomail.SendCloser, error) {
			return s.dial()
		},
	}
}

// Message representats an email message.
type Message struct {
	To               []string
	From             string
	Subject          string
	Body             map[string]string
	EmbeddedFiles    []string
	EmbeddedContents []EmbeddedContent
	ReplyTo          []string
	SingleEmail      bool
}

// SendEmail implements the EmailSender interface.
func (s *defaultEmailSender) SendEmail(_ context.Context, cmd *SendEmailSettings) error {
	message, err := s.buildEmailMessage(cmd)
	if err != nil {
		return err
	}

	_, err = s.Send(message)
	return err
}

func (s *defaultEmailSender) buildEmailMessage(cmd *SendEmailSettings) (*Message, error) {
	data := cmd.Data
	if data == nil {
		data = make(map[string]any, 10)
	}

	s.setDefaultTemplateData(data)

	body := make(map[string]string)
	for _, contentType := range s.cfg.ContentTypes {
		fileExtension, err := getFileExtensionByContentType(contentType)
		if err != nil {
			return nil, err
		}
		var buffer bytes.Buffer
		err = email.Template().ExecuteTemplate(&buffer, cmd.Template+fileExtension, data)
		if err != nil {
			return nil, err
		}

		body[contentType] = buffer.String()
	}

	subject := cmd.Subject
	if cmd.Subject == "" {
		subjectData := data["Subject"].(map[string]any)
		subjectText, hasSubject := subjectData["executed_template"].(string)
		if !hasSubject {
			return nil, fmt.Errorf("missing subject in template %s", cmd.Template)
		}
		subject = subjectText
	}

	addr := mail.Address{Name: s.cfg.FromName, Address: s.cfg.FromAddress}
	return &Message{
		To:               cmd.To,
		From:             addr.String(),
		Subject:          subject,
		Body:             body,
		EmbeddedFiles:    cmd.EmbeddedFiles,
		EmbeddedContents: cmd.EmbeddedContents,
		ReplyTo:          cmd.ReplyTo,
		SingleEmail:      cmd.SingleEmail,
	}, nil
}

func (s *defaultEmailSender) setDefaultTemplateData(data map[string]any) {
	data["AppUrl"] = s.cfg.ExternalURL
	data["Subject"] = map[string]any{}
	data["SentBy"] = s.cfg.SentBy
	dataCopy := map[string]any{}
	for k, v := range data {
		dataCopy[k] = v
	}
	data["TemplateData"] = dataCopy
}

func (s *defaultEmailSender) Send(messages ...*Message) (int, error) {
	sentEmailsCount := 0

	sender, err := s.dialFn(s)
	if err != nil {
		return sentEmailsCount, fmt.Errorf("failed to dial SMTP server: %w", err)
	}
	defer sender.Close()

	var errs error

	for _, msg := range messages {
		for _, m := range s.expandMsg(msg) {
			if err := gomail.Send(sender, m); err != nil {
				errs = errors.Join(errs, fmt.Errorf("failed to send notification to email addresses: %s: %w", strings.Join(msg.To, ";"), err))
			} else {
				sentEmailsCount++
			}
		}
	}

	return sentEmailsCount, errs
}

// expandMsg expands the message to a list of messages, one for each recipient
// if SingleEmail is false, otherwise it returns a single message.
func (s *defaultEmailSender) expandMsg(msg *Message) []*gomail.Message {
	if msg.SingleEmail || s.cfg.UseBCC {
		return []*gomail.Message{s.buildEmail(msg)}
	}

	result := make([]*gomail.Message, 0, len(msg.To))

	for _, recipient := range msg.To {
		msgCopy := *msg
		msgCopy.To = []string{recipient}
		m := s.buildEmail(&msgCopy)
		result = append(result, m)
	}

	return result
}

func (s *defaultEmailSender) dial() (gomail.SendCloser, error) {
	dialer, err := s.createDialer()
	if err != nil {
		return nil, err
	}
	if s.cfg.DialContext == nil {
		return dialer.Dial()
	}
	return s.dialWithContext(dialer)
}

// dialWithContext mirrors gomail.Dialer.Dial but opens the connection with cfg.DialContext.
func (s *defaultEmailSender) dialWithContext(d *gomail.Dialer) (_ gomail.SendCloser, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.Timeout)
	defer cancel()
	conn, err := s.cfg.DialContext(ctx, "tcp", net.JoinHostPort(d.Host, strconv.Itoa(d.Port)))
	if err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Now().Add(d.Timeout)); err != nil {
		conn.Close()
		return nil, err
	}
	if d.SSL {
		conn = tls.Client(conn, d.TLSConfig)
	}
	c, err := smtp.NewClient(conn, d.Host)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			c.Close()
		}
	}()

	if d.LocalName != "" {
		if err := c.Hello(d.LocalName); err != nil {
			return nil, err
		}
	}

	if !d.SSL && d.StartTLSPolicy != gomail.NoStartTLS {
		ok, _ := c.Extension("STARTTLS")
		if !ok && d.StartTLSPolicy == gomail.MandatoryStartTLS {
			return nil, gomail.StartTLSUnsupportedError{Policy: d.StartTLSPolicy}
		}
		if ok {
			if err := c.StartTLS(d.TLSConfig); err != nil {
				return nil, err
			}
		}
	}

	if d.Username != "" {
		if ok, auths := c.Extension("AUTH"); ok {
			var auth smtp.Auth
			switch {
			case strings.Contains(auths, "CRAM-MD5"):
				auth = smtp.CRAMMD5Auth(d.Username, d.Password)
			case strings.Contains(auths, "LOGIN") && !strings.Contains(auths, "PLAIN"):
				auth = &loginAuth{username: d.Username, password: d.Password, host: d.Host}
			default:
				auth = smtp.PlainAuth("", d.Username, d.Password, d.Host)
			}
			if err := c.Auth(auth); err != nil {
				return nil, err
			}
		}
	}

	return &smtpSender{Client: c, conn: conn, timeout: d.Timeout}, nil
}

// smtpSender is a gomail.SendCloser over a connection opened by dialWithContext.
type smtpSender struct {
	*smtp.Client
	conn    net.Conn
	timeout time.Duration
}

func (c *smtpSender) Send(from string, to []string, msg io.WriterTo) error {
	if err := c.conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return err
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err := c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := msg.WriteTo(w); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func (c *smtpSender) Close() error {
	return c.Quit()
}

// loginAuth is an smtp.Auth that implements the LOGIN mechanism.
type loginAuth struct {
	username, password, host string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && !slices.Contains(server.Auth, "LOGIN") {
		return "", nil, errors.New("unencrypted connection")
	}
	if server.Name != a.host {
		return "", nil, errors.New("wrong host name")
	}
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch string(fromServer) {
	case "Username:":
		return []byte(a.username), nil
	case "Password:":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected server challenge: %s", fromServer)
	}
}

func (s *defaultEmailSender) createDialer() (*gomail.Dialer, error) {
	host, port, err := net.SplitHostPort(s.cfg.Host)
	if err != nil {
		return nil, err
	}
	iPort, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	tlsconfig := &tls.Config{
		InsecureSkipVerify: s.cfg.SkipVerify,
		ServerName:         host,
	}

	if s.cfg.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(s.cfg.CertFile, s.cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("could not load cert or key file: %w", err)
		}
		tlsconfig.Certificates = []tls.Certificate{cert}
	}

	d := gomail.NewDialer(host, iPort, s.cfg.AuthUser, s.cfg.AuthPassword)
	d.TLSConfig = tlsconfig
	d.StartTLSPolicy = getStartTLSPolicy(s.cfg.StartTLSPolicy)
	d.LocalName = s.cfg.EhloIdentity

	return d, nil
}

func getStartTLSPolicy(policy string) gomail.StartTLSPolicy {
	switch policy {
	case "NoStartTLS":
		return -1
	case "MandatoryStartTLS":
		return 1
	default:
		return 0
	}
}

// buildEmail converts the Message DTO to a gomail message.
func (s *defaultEmailSender) buildEmail(msg *Message) *gomail.Message {
	m := gomail.NewMessage()
	// Add all static headers to the email message.
	for h, val := range s.cfg.StaticHeaders {
		m.SetHeader(h, val)
	}
	m.SetHeader("From", msg.From)
	if s.cfg.UseBCC && !msg.SingleEmail {
		m.SetHeader("Bcc", msg.To...)
	} else {
		m.SetHeader("To", msg.To...)
	}
	m.SetHeader("Subject", msg.Subject)

	// Add embedded files.
	for _, file := range msg.EmbeddedFiles {
		m.Embed(file)
	}

	for _, file := range msg.EmbeddedContents {
		m.Embed(file.Name, gomail.SetCopyFunc(func(writer io.Writer) error {
			_, err := writer.Write(file.Content)
			return err
		}))
	}

	// Add reply-to addresses to the email message.
	replyTo := make([]string, 0, len(msg.ReplyTo))
	for _, address := range msg.ReplyTo {
		replyTo = append(replyTo, m.FormatAddress(address, ""))
	}
	m.SetHeader("Reply-To", strings.Join(replyTo, ", "))

	// Loop over content types from settings in reverse order as they are ordered in according to descending
	// preference while the alternatives should be ordered according to ascending preference.
	for i := len(s.cfg.ContentTypes) - 1; i >= 0; i-- {
		if i == len(s.cfg.ContentTypes)-1 {
			m.SetBody(s.cfg.ContentTypes[i], msg.Body[s.cfg.ContentTypes[i]])
		} else {
			m.AddAlternative(s.cfg.ContentTypes[i], msg.Body[s.cfg.ContentTypes[i]])
		}
	}

	return m
}

func getFileExtensionByContentType(contentType string) (string, error) {
	switch contentType {
	case "text/html":
		return ".html", nil
	case "text/plain":
		return ".txt", nil
	default:
		return "", fmt.Errorf("unrecognized content type %q", contentType)
	}
}
