package email

// FullValidConfigForTesting a string representation of JSON object that contains all fields supported by the notifier Config. Can be used without secrets
const FullValidConfigForTesting = `{
	"addresses": "test@grafana.com", 
	"subject": "test-subject", 
	"message": "test-message", 
	"singleEmail": true
}`
