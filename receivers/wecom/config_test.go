package wecom

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
			expectedInitError: `either url or secret is required`,
		},
		{
			name:     "Minimal valid configuration (url)",
			settings: `{ "url" : "http://localhost"}`,
			expectedConfig: Config{
				Channel:     DefaultChannelType,
				EndpointURL: weComEndpoint,
				URL:         "http://localhost",
				AgentID:     "",
				CorpID:      "",
				Secret:      "",
				MsgType:     DefaultsgType,
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
		},
		{
			name:     "Minimal valid configuration (url) from secrets",
			settings: `{}`,
			secureSettings: map[string][]byte{
				"url": []byte("http://localhost"),
			},
			expectedConfig: Config{
				Channel:     DefaultChannelType,
				EndpointURL: weComEndpoint,
				URL:         "http://localhost",
				AgentID:     "",
				CorpID:      "",
				Secret:      "",
				MsgType:     DefaultsgType,
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
		},
		{
			name:     "Minimal valid configuration (secret)",
			settings: `{ "secret" : "test-secret", "agent_id" : "test-agent-id", "corp_id": "test-corp-id"}`,
			expectedConfig: Config{
				Channel:     "apiapp",
				EndpointURL: weComEndpoint,
				URL:         "",
				AgentID:     "test-agent-id",
				CorpID:      "test-corp-id",
				Secret:      "test-secret",
				MsgType:     DefaultsgType,
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
		},
		{
			name:     "Minimal valid configuration (secret) from secrets",
			settings: `{ "agent_id" : "test-agent-id", "corp_id": "test-corp-id"}`,
			secureSettings: map[string][]byte{
				"secret": []byte("test-secret"),
			},
			expectedConfig: Config{
				Channel:     "apiapp",
				EndpointURL: weComEndpoint,
				URL:         "",
				AgentID:     "test-agent-id",
				CorpID:      "test-corp-id",
				Secret:      "test-secret",
				MsgType:     DefaultsgType,
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
		},
		{
			name:     "should fallback to default if msgType is not known",
			settings: `{ "msgtype": "test-msg-type"}`,
			secureSettings: map[string][]byte{
				"url": []byte("http://localhost"),
			},
			expectedConfig: Config{
				Channel:     DefaultChannelType,
				EndpointURL: weComEndpoint,
				URL:         "http://localhost",
				AgentID:     "",
				CorpID:      "",
				Secret:      "",
				MsgType:     DefaultsgType,
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
		},
		{
			name:     "should parse message type 'markdown'",
			settings: `{ "msgtype": "markdown"}`,
			secureSettings: map[string][]byte{
				"url": []byte("http://localhost"),
			},
			expectedConfig: Config{
				Channel:     DefaultChannelType,
				EndpointURL: weComEndpoint,
				URL:         "http://localhost",
				AgentID:     "",
				CorpID:      "",
				Secret:      "",
				MsgType:     "markdown",
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
		},
		{
			name:     "should parse message type 'text'",
			settings: `{ "msgtype": "text"}`,
			secureSettings: map[string][]byte{
				"url": []byte("http://localhost"),
			},
			expectedConfig: Config{
				Channel:     DefaultChannelType,
				EndpointURL: weComEndpoint,
				URL:         "http://localhost",
				AgentID:     "",
				CorpID:      "",
				Secret:      "",
				MsgType:     "text",
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
		},
		{
			name: "All empty fields = minimal valid configuration",
			secureSettings: map[string][]byte{
				"url": []byte("http://localhost"),
			},
			settings: `{
				"endpointUrl" :"",
				"agent_id" :"",
				"corp_id" :"",
				"secret" :"",
				"msgtype" :"",
				"message" :"",
				"title" :"",
				"touser" :""
			}`,
			expectedConfig: Config{
				Channel:     DefaultChannelType,
				EndpointURL: weComEndpoint,
				URL:         "http://localhost",
				AgentID:     "",
				CorpID:      "",
				Secret:      "",
				MsgType:     DefaultsgType,
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
		},
		{
			name:     "Extracts all fields",
			settings: FullValidConfigForTesting,
			expectedConfig: Config{
				Channel:     DefaultChannelType,
				EndpointURL: "test-endpointUrl",
				URL:         "test-url",
				AgentID:     "test-agent_id",
				CorpID:      "test-corp_id",
				Secret:      "test-secret",
				MsgType:     "markdown",
				Message:     "test-message",
				Title:       "test-title",
				ToUser:      "test-touser",
			},
		},
		{
			name:           "Extracts all fields + override from secrets",
			secureSettings: receiversTesting.ReadSecretsJSONForTesting(FullValidSecretsForTesting),
			settings:       FullValidConfigForTesting,
			expectedConfig: Config{
				Channel:     DefaultChannelType,
				EndpointURL: "test-endpointUrl",
				URL:         "test-url-secret",
				AgentID:     "test-agent_id",
				CorpID:      "test-corp_id",
				Secret:      "test-secret",
				MsgType:     "markdown",
				Message:     "test-message",
				Title:       "test-title",
				ToUser:      "test-touser",
			},
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
