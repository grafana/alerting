package mqtt

// FullValidConfigForTesting is a string representation of a JSON object that contains all fields supported by the notifier Config. It can be used without secrets.
const FullValidConfigForTesting = `{
	"brokerUrl": "tcp://localhost:1883",
	"topic": "grafana/alerts",
	"messageFormat": "json",
	"clientId": "grafana-test-client-id",
	"username": "test-username",
	"insecureSkipVerify": false,
	"qos": 0,
	"retain": false,
	"password": "test-password",
	"tlsCACertificate": "test-tls-ca-certificate",
	"tlsClientCertificate": "test-tls-client-certificate",
	"tlsClientKey": "test-tls-client-key"
}`

// FullValidSecretsForTesting is a string representation of JSON object that contains all fields that can be overridden from secrets
const FullValidSecretsForTesting = `{
	"password": "test-password",
	"tlsCACertificate": "test-tls-ca-certificate",
	"tlsClientCertificate": "test-tls-client-certificate",
	"tlsClientKey": "test-tls-client-key"
}`
