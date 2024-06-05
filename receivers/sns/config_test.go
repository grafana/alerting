package sns

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
		secrets           map[string][]byte
		expected          Config
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
			expected: Config{
				APIUrl: "https://sns..amazonaws.com",
				Sigv4: SigV4Config{
					Profile: "default",
				},
				TopicARN: "arn:aws:sns:region:0123456789:SNSTopicName",
				Subject:  templates.DefaultMessageTitleEmbed,
				Message:  templates.DefaultMessageEmbed,
			},
		},
		{
			name: "Auth type set to keys if access key and secret key provided",
			settings: `{
				"topic_arn": "arn:aws:sns:region:0123456789:SNSTopicName",
				"sigv4": {
					"region": "us-east-1",
					"access_key": "access-key",
					"secret_key": "secret-key"
				}
			}`,
			expected: Config{
				APIUrl: "https://sns.us-east-1.amazonaws.com",
				Sigv4: SigV4Config{
					Region:    "us-east-1",
					AccessKey: "access-key",
					SecretKey: "secret-key",
				},
				TopicARN: "arn:aws:sns:region:0123456789:SNSTopicName",
				Subject:  templates.DefaultMessageTitleEmbed,
				Message:  templates.DefaultMessageEmbed,
			},
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
			expected: Config{
				APIUrl:   "https://sns..amazonaws.com",
				Sigv4:    SigV4Config{},
				TopicARN: "arn:aws:sns:region:0123456789:SNSTopicName",
				Subject:  templates.DefaultMessageTitleEmbed,
				Message:  templates.DefaultMessageEmbed,
			},
		},
		{
			name: "Subject and message are set by settings",
			settings: `{
				"topic_arn": "arn:aws:sns:region:0123456789:SNSTopicName",
				"subject": "subject",
				"message": "message"
			}`,
			expected: Config{
				APIUrl:   "https://sns..amazonaws.com",
				Sigv4:    SigV4Config{},
				TopicARN: "arn:aws:sns:region:0123456789:SNSTopicName",
				Subject:  "subject",
				Message:  "message",
			},
		},
		{
			name:     "Full config gives no errors",
			settings: FullValidConfigForTesting,
			expected: Config{
				Subject:     "subject",
				Message:     "message",
				APIUrl:      "https://sns.us-east-1.amazonaws.com",
				TopicARN:    "arn:aws:sns:us-east-1:0123456789:SNSTopicName",
				TargetARN:   "arn:aws:sns:us-east-1:0123456789:SNSTopicName",
				PhoneNumber: "123-456-7890",
				Attributes:  map[string]string{"attr1": "val1"},
				Sigv4: SigV4Config{
					Region:    "us-east-1",
					AccessKey: "access-key",
					SecretKey: "secret-key",
					Profile:   "default",
					RoleARN:   "arn:aws:iam:us-east-1:0123456789:role/my-role",
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sn, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(c.secrets))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.Equal(t, c.expected, sn)
		})
	}
}
