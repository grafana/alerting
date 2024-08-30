package mqtt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

type client interface {
	Connect(ctx context.Context, brokerURL, clientID, username, password string, tlsCfg *tls.Config) error
	Disconnect(ctx context.Context) error
	Publish(ctx context.Context, message message) error
}

type message struct {
	topic   string
	payload []byte
	retain  bool
	qos     int
}

type Notifier struct {
	*receivers.Base
	log      logging.Logger
	tmpl     *templates.Template
	settings Config
	client   client
}

func New(cfg Config, meta receivers.Metadata, template *templates.Template, logger logging.Logger, cli client) *Notifier {
	if cli == nil {
		cli = &mqttClient{}
	}

	return &Notifier{
		Base:     receivers.NewBase(meta),
		log:      logger,
		tmpl:     template,
		settings: cfg,
		client:   cli,
	}
}

// mqttMessage defines the JSON object send to an MQTT broker.
type mqttMessage struct {
	*templates.ExtendedData

	// The protocol version.
	Version  string `json:"version"`
	GroupKey string `json:"groupKey"`
	Message  string `json:"message"`
}

func (n *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	n.log.Debug("Sending an MQTT message", "topic", n.settings.Topic, "qos", n.settings.QoS, "retain", n.settings.Retain)

	msg, err := n.buildMessage(ctx, as...)
	if err != nil {
		n.log.Error("Failed to build MQTT message", "error", err.Error())
		return false, err
	}

	tlsCfg, err := n.buildTLSConfig()
	if err != nil {
		n.log.Error("Failed to build TLS config", "error", err.Error())
		return false, fmt.Errorf("failed to build TLS config: %s", err.Error())
	}

	err = n.client.Connect(ctx, n.settings.BrokerURL, n.settings.ClientID, n.settings.Username, n.settings.Password, tlsCfg)
	if err != nil {
		n.log.Error("Failed to connect to MQTT broker", "error", err.Error())
		return false, fmt.Errorf("Failed to connect to MQTT broker: %s", err.Error())
	}
	defer func() {
		err := n.client.Disconnect(ctx)
		if err != nil {
			n.log.Error("Failed to disconnect from MQTT broker", "error", err.Error())
		}
	}()

	qos, err := n.settings.QoS.Int64()
	if err != nil {
		n.log.Error("Failed to parse QoS", "error", err.Error())
		return false, fmt.Errorf("Failed to parse QoS: %s", err.Error())
	}

	err = n.client.Publish(
		ctx,
		message{
			topic:   n.settings.Topic,
			payload: []byte(msg),
			retain:  n.settings.Retain,
			qos:     int(qos),
		},
	)

	if err != nil {
		n.log.Error("Failed to publish MQTT message", "error", err.Error())
		return false, fmt.Errorf("Failed to publish MQTT message: %s", err.Error())
	}

	return true, nil
}

func (n *Notifier) buildTLSConfig() (*tls.Config, error) {
	if n.settings.TLSConfig == nil {
		return nil, nil
	}

	parsedURL, err := url.Parse(n.settings.BrokerURL)
	if err != nil {
		n.log.Error("Failed to parse broker URL", "error", err.Error())
		return nil, err
	}

	tlsCfg := &tls.Config{
		InsecureSkipVerify: n.settings.TLSConfig.InsecureSkipVerify,
		ServerName:         parsedURL.Hostname(),
	}

	if n.settings.TLSConfig.CACertificate != "" {
		tlsCfg.RootCAs = x509.NewCertPool()
		tlsCfg.RootCAs.AppendCertsFromPEM([]byte(n.settings.TLSConfig.CACertificate))
	}

	if n.settings.TLSConfig.ClientCertificate != "" || n.settings.TLSConfig.ClientKey != "" {
		cert, err := tls.X509KeyPair([]byte(n.settings.TLSConfig.ClientCertificate), []byte(n.settings.TLSConfig.ClientKey))
		if err != nil {
			n.log.Error("Failed to load client certificate", "error", err.Error())
			return nil, err
		}
		tlsCfg.Certificates = append(tlsCfg.Certificates, cert)
	}

	return tlsCfg, nil
}

func (n *Notifier) buildMessage(ctx context.Context, as ...*types.Alert) (string, error) {
	groupKey, err := notify.ExtractGroupKey(ctx)
	if err != nil {
		return "", err
	}

	var tmplErr error
	tmpl, data := templates.TmplText(ctx, n.tmpl, as, n.log, &tmplErr)
	messageText := tmpl(n.settings.Message)
	if tmplErr != nil {
		n.log.Warn("Failed to template MQTT message", "error", tmplErr.Error())
	}

	switch n.settings.MessageFormat {
	case MessageFormatText:
		return messageText, nil
	case MessageFormatJSON:
		msg := &mqttMessage{
			Version:      "1",
			ExtendedData: data,
			GroupKey:     groupKey.String(),
			Message:      messageText,
		}

		jsonMsg, err := json.Marshal(msg)
		if err != nil {
			return "", err
		}

		return string(jsonMsg), nil
	default:
		return "", errors.New("Invalid message format")
	}
}

func (n *Notifier) SendResolved() bool {
	return !n.GetDisableResolveMessage()
}
