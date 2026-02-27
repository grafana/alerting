package v0mimir1

import "github.com/prometheus/common/sigv4"

const FullValidConfigForTesting = ` {
	"http_config": {},
	"topic_arn": "arn:aws:sns:us-east-1:123456789012:alerts",
	"sigv4": {
		"Region": "us-east-1",
		"AccessKey": "secret-access-key",
		"SecretKey": "secret-secret-key",
		"Profile": "default",
		"RoleARN": "arn:aws:iam::123456789012:role/role-name"
	},
	"subject": "test subject",
	"message": "test message",
	"attributes": { "key1": "value1" },
	"send_resolved": true
}`

// GetFullValidConfig returns a fully populated Config struct with all fields
// set to non-zero values.
func GetFullValidConfig() Config {
	cfg := DefaultConfig
	cfg.APIUrl = "https://sns.us-east-1.amazonaws.com"
	cfg.Sigv4 = sigv4.SigV4Config{
		Region:  "us-east-1",
		Profile: "prod",
	}
	cfg.TopicARN = "arn:aws:sns:us-east-1:123456789:test"
	cfg.PhoneNumber = "+1234567890"
	cfg.TargetARN = "arn:aws:sns:us-east-1:123456789:endpoint"
	cfg.Subject = "Custom Subject"
	cfg.Message = "Custom Message"
	cfg.Attributes = map[string]string{"key": "value"}
	return cfg
}
