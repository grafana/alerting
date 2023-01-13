package config

import "encoding/json"

type NotificationChannelConfig struct {
	OrgID                 int64             // only used internally
	UID                   string            `json:"uid"`
	Name                  string            `json:"name"`
	Type                  string            `json:"type"`
	DisableResolveMessage bool              `json:"disableResolveMessage"`
	Settings              json.RawMessage   `json:"settings"`
	SecureSettings        map[string][]byte `json:"secureSettings"`
}
