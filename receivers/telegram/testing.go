package telegram

// FullValidConfigForTesting a string representation of JSON object that contains all fields supported by the notifier Config. Can be used without secrets
const FullValidConfigForTesting = `{
	"bottoken" :"test-token",
	"chatid" :"12345678",
	"message" :"test-message",
	"parse_mode" :"html",
	"disable_notifications" :true
}`

// FullValidSecretsForTesting is a string representation of JSON object that contains all fields that can be overridden from secrets
const FullValidSecretsForTesting = `{
	"bottoken": "test-secret-token"
}`
