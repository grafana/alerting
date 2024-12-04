package oncall_ng

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

type Config struct {
	APIURL                   string `json:"api_url"`
	AuthorizationScheme      string `json:"authorization_scheme,omitempty"`
	AuthorizationCredentials string `json:"authorization_credentials,omitempty"`
	EscalationChainID        string `json:"escalation_chain_id"`
	Title                    string `json:"title"`
	Message                  string `json:"message"`
}

func NewConfig(jsonData json.RawMessage, decryptFn receivers.DecryptFunc) (Config, error) {
	settings := Config{}
	err := json.Unmarshal(jsonData, &settings)
	if err != nil {
		return settings, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	if settings.EscalationChainID == "" {
		return settings, errors.New("required field 'escalation_chain_id' is not specified")
	}
	if settings.APIURL == "" {
		return settings, errors.New("required field 'api_url' is not specified")
	}
	_, err = url.Parse(settings.APIURL)
	if err != nil {
		return settings, fmt.Errorf("failed to parse URL %s: %w", settings.APIURL, err)
	}
	if settings.Title == "" {
		settings.Title = templates.DefaultMessageTitleEmbed
	}
	if settings.Message == "" {
		settings.Message = templates.DefaultMessageEmbed
	}
	return settings, err
}
