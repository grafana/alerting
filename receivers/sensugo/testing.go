package sensugo

// FullValidConfigForTesting a string representation of JSON object that contains all fields supported by the notifier Config. Can be used without secrets
const FullValidConfigForTesting = `{
	"url": "http://localhost",  
	"apikey": "test-api-key",
	"entity" : "test-entity",
	"check" : "test-check",
	"namespace" : "test-namespace",
	"handler" : "test-handler",
	"message" : "test-message"
}`

// FullValidSecretsForTesting is a string representation of JSON object that contains all fields that can be overridden from secrets
const FullValidSecretsForTesting = `{
	"apikey": "test-secret-api-key"
}`
