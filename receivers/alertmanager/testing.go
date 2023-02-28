package alertmanager

// FullValidConfigForTesting a string representation of JSON object that contains all fields supported by the notifier Config. Can be used without secrets
const FullValidConfigForTesting = `{
	"url": "https://alertmanager-01.com",
	"basicAuthUser": "grafana",
	"basicAuthPassword": "admin"
}`

// FullValidSecretsForTesting is a string representation of JSON object that contains all fields that can be overridden from secrets
const FullValidSecretsForTesting = `{
	"basicAuthPassword": "grafana-admin"
}`
