package slack

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
	testing2 "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

func TestValidateConfig(t *testing.T) {
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
			expectedInitError: `recipient must be specified when using the Slack chat API`,
		},
		{
			name:              "Error if default URL and recipient is missing",
			settings:          `{ "token": "test-token"}`,
			expectedInitError: `recipient must be specified when using the Slack chat API`,
		},
		{
			name:              "Error if default URL and token is missing",
			settings:          `{ "recipient" : "test-recipient" }`,
			expectedInitError: `token must be specified when using the Slack chat API`,
		},
		{
			name:     "Minimal valid configuration (ChatAPI)",
			settings: `{ "recipient" : "test-recipient", "token": "test-token"}`,
			expectedConfig: Config{
				EndpointURL:    APIURL,
				URL:            APIURL,
				Token:          "test-token",
				Recipient:      "test-recipient",
				Text:           templates.DefaultMessageEmbed,
				Title:          templates.DefaultMessageTitleEmbed,
				Username:       "Grafana",
				IconEmoji:      "",
				IconURL:        "",
				MentionChannel: "",
				MentionUsers:   nil,
				MentionGroups:  nil,
			},
		},
		{
			name:     "Minimal valid configuration (ChatAPI) from secrets",
			settings: `{ "recipient" : "test-recipient" }`,
			secureSettings: map[string][]byte{
				"token": []byte("test-token"),
			},
			expectedConfig: Config{
				EndpointURL:    APIURL,
				URL:            APIURL,
				Token:          "test-token",
				Recipient:      "test-recipient",
				Text:           templates.DefaultMessageEmbed,
				Title:          templates.DefaultMessageTitleEmbed,
				Username:       "Grafana",
				IconEmoji:      "",
				IconURL:        "",
				MentionChannel: "",
				MentionUsers:   nil,
				MentionGroups:  nil,
			},
		},
		{
			name:     "Minimal valid configuration (WebhookAPI)",
			settings: `{ "url" : "http://slack.local/some-webhook"}`,
			expectedConfig: Config{
				EndpointURL:    APIURL,
				URL:            "http://slack.local/some-webhook",
				Token:          "",
				Recipient:      "",
				Text:           templates.DefaultMessageEmbed,
				Title:          templates.DefaultMessageTitleEmbed,
				Username:       "Grafana",
				IconEmoji:      "",
				IconURL:        "",
				MentionChannel: "",
				MentionUsers:   nil,
				MentionGroups:  nil,
			},
		},
		{
			name:     "Minimal valid configuration (WebhookAPI) from secrets",
			settings: `{ }`,
			secureSettings: map[string][]byte{
				"url": []byte("http://slack.local/some-webhook"),
			},
			expectedConfig: Config{
				EndpointURL:    APIURL,
				URL:            "http://slack.local/some-webhook",
				Token:          "",
				Recipient:      "",
				Text:           templates.DefaultMessageEmbed,
				Title:          templates.DefaultMessageTitleEmbed,
				Username:       "Grafana",
				IconEmoji:      "",
				IconURL:        "",
				MentionChannel: "",
				MentionUsers:   nil,
				MentionGroups:  nil,
			},
		},
		{
			name:              "Should error if URL is not valid",
			settings:          `{ "url" : "://slack.local/some-webhook"}`,
			expectedInitError: `invalid URL "://slack.local/some-webhook"`,
		},
		{
			name:     "Should error if URL from secrets is not valid ",
			settings: `{ }`,
			secureSettings: map[string][]byte{
				"url": []byte("://slack.local/some-webhook"),
			},
			expectedInitError: `invalid URL "://slack.local/some-webhook"`,
		},
		{
			name:     "Should overwrite token from secrets",
			settings: `{"url": "http://localhost", "token": "test" }`,
			secureSettings: map[string][]byte{
				"url":   []byte("http://slack.local/some-webhook"),
				"token": []byte("test-token"),
			},
			expectedConfig: Config{
				EndpointURL:    APIURL,
				URL:            "http://slack.local/some-webhook",
				Token:          "test-token",
				Recipient:      "",
				Text:           templates.DefaultMessageEmbed,
				Title:          templates.DefaultMessageTitleEmbed,
				Username:       "Grafana",
				IconEmoji:      "",
				IconURL:        "",
				MentionChannel: "",
				MentionUsers:   nil,
				MentionGroups:  nil,
			},
		},
		{
			name:     "Should error if mention is not supported",
			settings: `{ "recipient" : "test-recipient" , "mentionChannel": "test-channel" }`,
			secureSettings: map[string][]byte{
				"token": []byte("test-token"),
			},
			expectedInitError: `invalid value for mentionChannel: "test-channel"`,
		},
		{
			name:     "Should accept mention \"here\"",
			settings: `{ "recipient" : "test-recipient" , "mentionChannel": "here" }`,
			secureSettings: map[string][]byte{
				"token": []byte("test-token"),
			},
			expectedConfig: Config{
				EndpointURL:    APIURL,
				URL:            APIURL,
				Token:          "test-token",
				Recipient:      "test-recipient",
				Text:           templates.DefaultMessageEmbed,
				Title:          templates.DefaultMessageTitleEmbed,
				Username:       "Grafana",
				IconEmoji:      "",
				IconURL:        "",
				MentionChannel: "here",
				MentionUsers:   nil,
				MentionGroups:  nil,
			},
		},
		{
			name:     "Should accept mention \"channel\"",
			settings: `{ "recipient" : "test-recipient" , "mentionChannel": "channel" }`,
			secureSettings: map[string][]byte{
				"token": []byte("test-token"),
			},
			expectedConfig: Config{
				EndpointURL:    APIURL,
				URL:            APIURL,
				Token:          "test-token",
				Recipient:      "test-recipient",
				Text:           templates.DefaultMessageEmbed,
				Title:          templates.DefaultMessageTitleEmbed,
				Username:       "Grafana",
				IconEmoji:      "",
				IconURL:        "",
				MentionChannel: "channel",
				MentionUsers:   nil,
				MentionGroups:  nil,
			},
		},
		{
			name:     "Should parse mentionUsers",
			settings: `{ "recipient" : "test-recipient" , "mentionUsers": "user-1,user-2,user-3" }`,
			secureSettings: map[string][]byte{
				"token": []byte("test-token"),
			},
			expectedConfig: Config{
				EndpointURL:    APIURL,
				URL:            APIURL,
				Token:          "test-token",
				Recipient:      "test-recipient",
				Text:           templates.DefaultMessageEmbed,
				Title:          templates.DefaultMessageTitleEmbed,
				Username:       "Grafana",
				IconEmoji:      "",
				IconURL:        "",
				MentionChannel: "",
				MentionUsers: []string{
					"user-1",
					"user-2",
					"user-3",
				},
				MentionGroups: nil,
			},
		},
		{
			name:     "Should parse mentionGroups",
			settings: `{ "recipient" : "test-recipient" , "mentionGroups": "users-1,users-2,users-3" }`,
			secureSettings: map[string][]byte{
				"token": []byte("test-token"),
			},
			expectedConfig: Config{
				EndpointURL:    APIURL,
				URL:            APIURL,
				Token:          "test-token",
				Recipient:      "test-recipient",
				Text:           templates.DefaultMessageEmbed,
				Title:          templates.DefaultMessageTitleEmbed,
				Username:       "Grafana",
				IconEmoji:      "",
				IconURL:        "",
				MentionChannel: "",
				MentionUsers:   nil,
				MentionGroups: []string{
					"users-1",
					"users-2",
					"users-3",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &receivers.NotificationChannelConfig{
				Settings:       json.RawMessage(c.settings),
				SecureSettings: c.secureSettings,
			}
			fc, err := testing2.NewFactoryConfigForValidateConfigTesting(t, m)
			require.NoError(t, err)

			actual, err := ValidateConfig(fc)

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
