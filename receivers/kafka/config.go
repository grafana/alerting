package kafka

import (
	"context"
	"encoding/json"
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
	KafkaAPIVersionV2 = "v2"
	KafkaAPIVersionV3 = "v3"
)

type KafkaConfig struct {
	Endpoint       string `json:"kafkaRestProxy,omitempty" yaml:"kafkaRestProxy,omitempty"`
	Topic          string `json:"kafkaTopic,omitempty" yaml:"kafkaTopic,omitempty"`
	Description    string `json:"description,omitempty" yaml:"description,omitempty"`
	Details        string `json:"details,omitempty" yaml:"details,omitempty"`
	Username       string `json:"username,omitempty" yaml:"username,omitempty"`
	Password       string `json:"password,omitempty" yaml:"password,omitempty"`
	APIVersion     string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	KafkaClusterID string `json:"kafkaClusterId,omitempty" yaml:"kafkaClusterId,omitempty"`
}

func BuildKafkaConfig(fc receivers.FactoryConfig) (*KafkaConfig, error) {
	var settings KafkaConfig
	err := json.Unmarshal(fc.Config.Settings, &settings)
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
	settings.Password = fc.DecryptFunc(context.Background(), fc.Config.SecureSettings, "password", settings.Password)

	if settings.APIVersion == "" {
		settings.APIVersion = KafkaAPIVersionV2
	} else if settings.APIVersion == KafkaAPIVersionV3 {
		if settings.KafkaClusterID == "" {
			return nil, errors.New("kafka cluster id must be provided when using api version 3")
		}
	} else if settings.APIVersion != KafkaAPIVersionV2 && settings.APIVersion != KafkaAPIVersionV3 {
		return nil, fmt.Errorf("unsupported api version: %s", settings.APIVersion)
	}
	return &settings, nil
}
