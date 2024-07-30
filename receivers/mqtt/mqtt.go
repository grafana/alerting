package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"

	mqttLib "github.com/eclipse/paho.mqtt.golang"
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

func (n *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	n.log.Debug("Sending an MQTT message")

	if token := n.client.Connect(); token.Wait() && token.Error() != nil {
		n.log.Error("Failed to connect to MQTT broker", "error", token.Error())
		return false, fmt.Errorf("Failed to connect to MQTT broker: %w", token.Error())
	}

	var err error
	tmpl, _ := templates.TmplText(ctx, n.tmpl, as, n.log, &err)
	messageText := tmpl(n.settings.Message)
	if err != nil {
		n.log.Error("Failed to template MQTT message", "error", err)
		return false, fmt.Errorf("Failed to template MQTT message: %w", err)
	}

	if token := n.client.Publish(n.settings.Topic, 0, false, messageText); token.Wait() && token.Error() != nil {
		n.log.Error("Failed to publish MQTT message", "error", token.Error())
		return false, fmt.Errorf("Failed to publish MQTT message: %w", token.Error())
	}

	n.client.Disconnect(250)

	return true, nil
}

func (n *Notifier) SendResolved() bool {
	return !n.GetDisableResolveMessage()
}
