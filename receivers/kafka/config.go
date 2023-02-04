package kafka

import (
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

// The user can choose which API version to use when sending
// messages to Kafka. The default is v2.
// Details on how these versions differ can be found here:
// https://docs.confluent.io/platform/current/kafka-rest/api.html
const (
	apiVersionV2 = "v2"
	apiVersionV3 = "v3"
)

type Config struct {
	Endpoint       string           `json:"kafkaRestProxy,omitempty" yaml:"kafkaRestProxy,omitempty"`
	Topic          string           `json:"kafkaTopic,omitempty" yaml:"kafkaTopic,omitempty"`
	Description    string           `json:"description,omitempty" yaml:"description,omitempty"`
	Details        string           `json:"details,omitempty" yaml:"details,omitempty"`
	Username       string           `json:"username,omitempty" yaml:"username,omitempty"`
	Password       receivers.Secret `json:"password,omitempty" yaml:"password,omitempty"`
	APIVersion     string           `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	KafkaClusterID string           `json:"kafkaClusterId,omitempty" yaml:"kafkaClusterId,omitempty"`
}

func ValidateConfig(fc receivers.FactoryConfig) (*Config, error) {
	var settings Config
	err := fc.Marshaller.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.Endpoint == "" {
		return nil, errors.New("could not find kafka rest proxy endpoint property in settings")
	}
	settings.Endpoint = strings.TrimRight(settings.Endpoint, "/")

	if settings.Topic == "" {
		return nil, errors.New("could not find kafka topic property in settings")
	}
	if settings.Description == "" {
		settings.Description = templates.DefaultMessageTitleEmbed
	}
	if settings.Details == "" {
		settings.Details = templates.DefaultMessageEmbed
	}

	if settings.APIVersion == "" {
		settings.APIVersion = apiVersionV2
	} else if settings.APIVersion == apiVersionV3 {
		if settings.KafkaClusterID == "" {
			return nil, errors.New("kafka cluster id must be provided when using api version 3")
		}
	} else if settings.APIVersion != apiVersionV2 && settings.APIVersion != apiVersionV3 {
		return nil, fmt.Errorf("unsupported api version: %s", settings.APIVersion)
	}
	return &settings, nil
}
