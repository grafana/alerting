package line

// FullValidConfigForTesting a string representation of JSON object that contains all fields supported by the notifier Config. Can be used without secrets
const FullValidConfigForTesting = `{
	"token": "test", 
	"title": "test-title", 
	"description": "test-description" 
}`

// FullValidSecretsForTesting is a string representation of JSON object that contains all fields that can be overridden from secrets
const FullValidSecretsForTesting = `{
	"token": "test-secret-token"
}`
