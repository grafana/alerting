package kafka

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

func TestNewConfig(t *testing.T) {
	cases := []struct {
		name              string
		settings          string
		secureSettings    map[string][]byte
		expectedConfig    Config
		expectedInitError string
	}{
		{
			name:              "Error if empty",
			settings:          "",
			expectedInitError: `failed to unmarshal settings`,
		},
		{
			name:              "Error if empty JSON object",
			settings:          `{}`,
			expectedInitError: `could not find kafka rest proxy endpoint property in settings`,
		},
		{
			name:     "Minimal valid configuration",
			settings: `{"kafkaRestProxy": "http://localhost", "kafkaTopic" : "test-topic"}`,
			expectedConfig: Config{
				Endpoint:       "http://localhost",
				Topic:          "test-topic",
				Description:    templates.DefaultMessageTitleEmbed,
				Details:        templates.DefaultMessageEmbed,
				Username:       "",
				Password:       "",
				APIVersion:     apiVersionV2,
				KafkaClusterID: "",
			},
		},
		{
			name:              "Error if Endpoint is empty",
			settings:          `{ "kafkaTopic" : "test-topic" }`,
			expectedInitError: `could not find kafka rest proxy endpoint property in settings`,
		},
		{
			name:              "Error if Topic is empty",
			settings:          `{ "kafkaRestProxy": "http://localhost" }`,
			expectedInitError: `could not find kafka topic property in settings`,
		},
		{
			name:     "Should trim leading slash from endpoint",
			settings: `{"kafkaRestProxy": "http://localhost/", "kafkaTopic" : "test-topic"}`,
			expectedConfig: Config{
				Endpoint:       "http://localhost",
				Topic:          "test-topic",
				Description:    templates.DefaultMessageTitleEmbed,
				Details:        templates.DefaultMessageEmbed,
				Username:       "",
				Password:       "",
				APIVersion:     apiVersionV2,
				KafkaClusterID: "",
			},
		},
		{
			name:     "Should decrypt password",
			settings: `{"kafkaRestProxy": "http://localhost/", "kafkaTopic" : "test-topic"}`,
			secureSettings: map[string][]byte{
				"password": []byte("test-password"),
			},
			expectedConfig: Config{
				Endpoint:       "http://localhost",
				Topic:          "test-topic",
				Description:    templates.DefaultMessageTitleEmbed,
				Details:        templates.DefaultMessageEmbed,
				Username:       "",
				Password:       "test-password",
				APIVersion:     apiVersionV2,
				KafkaClusterID: "",
			},
		},
		{
			name: "All empty fields = minimal valid configuration",
			settings: `{
				"kafkaRestProxy": "http://localhost/", 
				"kafkaTopic" : "test-topic", 
				"description" : "", 
				"details": "", 
				"username": "", 
				"password": "", 
				"apiVersion": "", 
				"kafkaClusterId": ""
			}`,
			expectedConfig: Config{
				Endpoint:       "http://localhost",
				Topic:          "test-topic",
				Description:    templates.DefaultMessageTitleEmbed,
				Details:        templates.DefaultMessageEmbed,
				Username:       "",
				Password:       "",
				APIVersion:     apiVersionV2,
				KafkaClusterID: "",
			},
		},
		{
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				Endpoint:       "http://localhost",
				Topic:          "test-topic",
				Description:    "test-description",
				Details:        "test-details",
				Username:       "test-user",
				Password:       "password",
				APIVersion:     "v2",
				KafkaClusterID: "12345",
			},
		},
		{
			name:           "Should override password from secrets",
			settings:       FullValidConfigForTesting,
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			expectedConfig: Config{
				Endpoint:       "http://localhost",
				Topic:          "test-topic",
				Description:    "test-description",
				Details:        "test-details",
				Username:       "test-user",
				Password:       "test-password",
				APIVersion:     "v2",
				KafkaClusterID: "12345",
			},
		},
		{
			name: "Error if api version is unknown",
			settings: `{
				"kafkaRestProxy": "http://localhost/", 
				"kafkaTopic" : "test-topic", 
				"apiVersion": "test-1235" 
			}`,
			expectedInitError: "unsupported api version: test-1235",
		},
		{
			name: "Error if clusterId is not specified for api version 3",
			settings: `{
				"kafkaRestProxy": "http://localhost/", 
				"kafkaTopic" : "test-topic", 
				"apiVersion": "v3" 
			}`,
			expectedInitError: "kafka cluster id must be provided when using api version 3",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(c.secureSettings))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
