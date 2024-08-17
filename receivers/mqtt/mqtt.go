package mqtt

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"

	mqttLib "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

type Client interface {
	Connect() mqttLib.Token
	Publish(topic string, qos byte, retained bool, payload interface{}) mqttLib.Token
	Disconnect(quiesce uint)
}

type Notifier struct {
	*receivers.Base
	log      logging.Logger
	tmpl     *templates.Template
	settings Config
	client   Client
}

func defaultClientFactory(opts *mqttLib.ClientOptions) Client {
	return mqttLib.NewClient(opts)
}

func New(cfg Config, meta receivers.Metadata, template *templates.Template, logger logging.Logger, clientFactory func(opts *mqttLib.ClientOptions) Client) *Notifier {
	if clientFactory == nil {
		clientFactory = defaultClientFactory
	}

	opts := mqttLib.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(cfg.ClientID).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password)

	if cfg.InsecureSkipVerify {
		tlsCfg := tls.Config{
			InsecureSkipVerify: true,
		}
		opts.SetTLSConfig(&tlsCfg)
	}

	return &Notifier{
		Base:     receivers.NewBase(meta),
		log:      logger,
		tmpl:     template,
		settings: cfg,
		client:   clientFactory(opts),
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
	n.log.Debug("Sending an MQTT message")

	msg, err := n.buildMessage(ctx, as...)
	if err != nil {
		return false, err
	}

	if token := n.client.Connect(); token.Wait() && token.Error() != nil {
		n.log.Error("Failed to connect to MQTT broker", "error", token.Error())
		return false, fmt.Errorf("Failed to connect to MQTT broker: %w", token.Error())
	}
	defer n.client.Disconnect(250)

	if token := n.client.Publish(n.settings.Topic, 0, false, string(msg)); token.Wait() && token.Error() != nil {
		n.log.Error("Failed to publish MQTT message", "error", token.Error())
		return false, fmt.Errorf("Failed to publish MQTT message: %w", token.Error())
	}

	return true, nil
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
