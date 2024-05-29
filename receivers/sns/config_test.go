package sns

import (
	"encoding/json"
	"github.com/grafana/alerting/templates"
	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	"testing"

	"github.com/stretchr/testify/require"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestNewConfig(t *testing.T) {
	cases := []struct {
		name              string
		settings          string
		secrets           map[string][]byte
		expectedSubject   string
		expectedMessage   string
		expectedAuthType  string
		expectedInitError string
	}{
		{
			name:              "Error if empty JSON object",
			settings:          `{}`,
			expectedInitError: "must specify topicArn, targetArn, or phone number",
		},
		{
			name: "Auth type is set to credentials profile if profile provided",
			settings: `{
				"topic_arn": "arn:aws:sns:region:0123456789:SNSTopicName",
				"sigv4": {
					"profile": "default"
				}
			}`,
			expectedAuthType: awsds.AuthTypeSharedCreds.String(),
			expectedSubject:  templates.DefaultMessageTitleEmbed,
			expectedMessage:  templates.DefaultMessageEmbed,
		},
		{
			name: "Auth type set to keys if access key and secret key provided",
			settings: `{
				"topic_arn": "arn:aws:sns:region:0123456789:SNSTopicName",
				"sigv4": {
					"access_key": "access-key",
					"secret_key": "secret-key"
				}
			}`,
			expectedAuthType: awsds.AuthTypeKeys.String(),
			expectedSubject:  templates.DefaultMessageTitleEmbed,
			expectedMessage:  templates.DefaultMessageEmbed,
		},
		{
			name: "Validation fails if missing secret key",
			settings: `{
				"topic_arn": "arn:aws:sns:region:0123456789:SNSTopicName",
				"sigv4": {
					"access_key": "access-key"
				}
			}`,
			expectedInitError: "must specify both access key and secret key",
		},
		{
			name: "Validation fails if missing access key",
			settings: `{
				"topic_arn": "arn:aws:sns:region:0123456789:SNSTopicName",
				"sigv4": {
					"secret_key": "secret-key"
				}
			}`,
			expectedInitError: "must specify both access key and secret key",
		},
		{
			name: "Auth type set to default if keys and profile not provided",
			settings: `{
				"topic_arn": "arn:aws:sns:region:0123456789:SNSTopicName"
			}`,
			expectedAuthType: awsds.AuthTypeDefault.String(),
			expectedSubject:  templates.DefaultMessageTitleEmbed,
			expectedMessage:  templates.DefaultMessageEmbed,
		},
		{
			name: "Subject and message are set by settings",
			settings: `{
				"topic_arn": "arn:aws:sns:region:0123456789:SNSTopicName",
				"subject": "subject",
				"message": "message"
			}`,
			expectedSubject:  "subject",
			expectedMessage:  "message",
			expectedAuthType: awsds.AuthTypeDefault.String(),
		},
		{
			name:             "Full config gives no errors",
			settings:         FullValidConfigForTesting,
			expectedSubject:  "subject",
			expectedMessage:  "message",
			expectedAuthType: awsds.AuthTypeSharedCreds.String(),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sn, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(c.secrets))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}

			require.Equal(t, c.expectedSubject, sn.Subject)
			require.Equal(t, c.expectedMessage, sn.Message)
			require.Equal(t, c.expectedAuthType, sn.AWSAuthSettings.AuthType.String())
		})
	}
}
