package dinding

// FullValidConfigForTesting a string representation of JSON object that contains all fields supported by the notifier Config. Can be used without secrets
const FullValidConfigForTesting = `{
	"url": "http://localhost",
	"message": "{{ len .Alerts.Firing }} alerts are firing, {{ len .Alerts.Resolved }} are resolved",
    "title": "Alerts firing: {{ len .Alerts.Firing }}",
	"msgType": "actionCard"
}`
