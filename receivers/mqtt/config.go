package mqtt

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

type Config struct {
	BrokerURL          string `json:"brokerUrl,omitempty" yaml:"brokerUrl,omitempty"`
	ClientID           string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	Topic              string `json:"topic,omitempty" yaml:"topic,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Username           string `json:"username,omitempty" yaml:"username,omitempty"`
	Password           string `json:"password,omitempty" yaml:"password,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
}

func NewConfig(jsonData json.RawMessage, decryptFn receivers.DecryptFunc) (Config, error) {
	var settings Config
	err := json.Unmarshal(jsonData, &settings)
	if err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.BrokerURL == "" {
		return Config{}, errors.New("MQTT broker URL must be specified")
	}
	if _, err := isValidMqttURL(settings.BrokerURL); err != nil {
		return Config{}, fmt.Errorf("Invalid MQTT broker URL: %w", err)
	}

	if settings.Topic == "" {
		return Config{}, errors.New("MQTT topic must be specified")
	}

	if settings.ClientID == "" {
		settings.ClientID = "Grafana"
	}

	if settings.Message == "" {
		settings.Message = templates.DefaultMessageEmbed
	}

	password := decryptFn("password", settings.Password)
	settings.Password = password

	return settings, nil
}

func isValidMqttURL(mqttURL string) (bool, error) {
	parsedURL, err := url.Parse(mqttURL)
	if err != nil {
		return false, err
	}

	if parsedURL.Scheme != "tcp" && parsedURL.Scheme != "ssl" {
		return false, errors.New("Invalid scheme, must be 'tcp' or 'ssl'")
	}

	host := parsedURL.Host
	if !strings.Contains(host, ":") {
		return false, errors.New("Port must be specified")
	}

	_, port, err := net.SplitHostPort(host)
	if err != nil {
		return false, err
	}

	if portNum, err := strconv.ParseInt(port, 10, 32); err != nil || portNum > 65535 || portNum < 1 {
		return false, errors.New("Port must be a valid number between 1 and 65535")
	}

	return true, nil
}
