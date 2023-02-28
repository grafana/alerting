package opsgenie

// FullValidConfigForTesting a string representation of JSON object that contains all fields supported by the notifier Config. Can be used without secrets
const FullValidConfigForTesting = `{
	"apiUrl" : "http://localhost", 
	"apiKey": "test-api-key",
	"message" : "test-message", 
	"description": "test-description", 
	"autoClose": false, 
	"overridePriority": false, 
	"sendTagsAs": "both"
}`

// FullValidSecretsForTesting is a string representation of JSON object that contains all fields that can be overridden from secrets
const FullValidSecretsForTesting = `{
	"apiKey": "test-secret-api-key"
}`
