package v1

// FullValidConfigForTesting is a string representation of a JSON object that contains all fields supported by the notifier Config. It can be used without secrets.
const FullValidConfigForTesting = `{
	"homeserverUrl": "https://matrix.example.com",
	"accessToken": "test-token",
	"roomId": "!abc:example.com",
	"messageType": "m.text",
	"title": "test-title",
	"message": "test-message"
}`

// FullValidSecretsForTesting is a string representation of a JSON object that contains secret fields of the notifier Config.
const FullValidSecretsForTesting = `{
	"accessToken": "test-token"
}`
