package v0mimir1

const FullValidConfigForTesting = `{
	"api_url": "http://localhost",
	"project": "PROJ",
	"issue_type": "Bug",
	"summary": {"template": "test summary"},
	"description": {"template": "test description"},
	"priority": "High",
	"labels": ["alertmanager"],
	"custom_fields": {
		"customfield_10000": "test customfield_10000"
	},
	"send_resolved": true
}`
