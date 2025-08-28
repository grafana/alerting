// copy of https://github.com/grafana/mimir/blob/a5f6bc75e858f2f7ede4b68bd692ed9b4f99193d/pkg/alertmanager/api_test.go

package definition

import (
	"testing"

	"github.com/prometheus/alertmanager/config"
	commoncfg "github.com/prometheus/common/config"
	"github.com/stretchr/testify/assert"
)

func TestValidateAlertmanagerConfig(t *testing.T) {
	tests := map[string]struct {
		input    interface{}
		expected error
	}{
		"*HTTPClientConfig": {
			input: &commoncfg.HTTPClientConfig{
				BasicAuth: &commoncfg.BasicAuth{
					PasswordFile: "/secrets",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"HTTPClientConfig": {
			input: commoncfg.HTTPClientConfig{
				BasicAuth: &commoncfg.BasicAuth{
					PasswordFile: "/secrets",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"*TLSConfig": {
			input: &commoncfg.TLSConfig{
				CertFile: "/cert",
			},
			expected: errTLSConfigNotAllowed,
		},
		"TLSConfig": {
			input: commoncfg.TLSConfig{
				CertFile: "/cert",
			},
			expected: errTLSConfigNotAllowed,
		},
		"*GlobalConfig.SMTPAuthPasswordFile": {
			input: &config.GlobalConfig{
				SMTPAuthPasswordFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"GlobalConfig.SMTPAuthPasswordFile": {
			input: config.GlobalConfig{
				SMTPAuthPasswordFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"*DiscordConfig.HTTPConfig": {
			input: &config.DiscordConfig{
				HTTPConfig: &commoncfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"DiscordConfig.HTTPConfig": {
			input: &config.DiscordConfig{
				HTTPConfig: &commoncfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"*DiscordConfig.WebhookURLFile": {
			input: &config.DiscordConfig{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"DiscordConfig.WebhookURLFile": {
			input: config.DiscordConfig{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"*EmailConfig.AuthPasswordFile": {
			input: &config.EmailConfig{
				AuthPasswordFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"EmailConfig.AuthPasswordFile": {
			input: config.EmailConfig{
				AuthPasswordFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"*MSTeams.HTTPConfig": {
			input: &config.MSTeamsConfig{
				HTTPConfig: &commoncfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"MSTeams.HTTPConfig": {
			input: &config.MSTeamsConfig{
				HTTPConfig: &commoncfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"*MSTeams.WebhookURLFile": {
			input: &config.MSTeamsConfig{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"MSTeams.WebhookURLFile": {
			input: config.MSTeamsConfig{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"struct containing *HTTPClientConfig as direct child": {
			input: config.GlobalConfig{
				HTTPConfig: &commoncfg.HTTPClientConfig{
					BasicAuth: &commoncfg.BasicAuth{
						PasswordFile: "/secrets",
					},
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"struct containing *HTTPClientConfig as nested child": {
			input: config.Config{
				Global: &config.GlobalConfig{
					HTTPConfig: &commoncfg.HTTPClientConfig{
						BasicAuth: &commoncfg.BasicAuth{
							PasswordFile: "/secrets",
						},
					},
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"struct containing *HTTPClientConfig as nested child within a slice": {
			input: config.Config{
				Receivers: []config.Receiver{{
					Name: "test",
					WebhookConfigs: []*config.WebhookConfig{{
						HTTPConfig: &commoncfg.HTTPClientConfig{
							BasicAuth: &commoncfg.BasicAuth{
								PasswordFile: "/secrets",
							},
						},
					}}},
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"map containing *HTTPClientConfig": {
			input: map[string]*commoncfg.HTTPClientConfig{
				"test": {
					BasicAuth: &commoncfg.BasicAuth{
						PasswordFile: "/secrets",
					},
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"map containing TLSConfig as nested child": {
			input: map[string][]config.EmailConfig{
				"test": {{
					TLSConfig: commoncfg.TLSConfig{
						CAFile: "/file",
					},
				}},
			},
			expected: errTLSConfigNotAllowed,
		},
	}

	for testName, testData := range tests {
		t.Run(testName, func(t *testing.T) {
			err := ValidateAlertmanagerConfig(testData.input)
			assert.ErrorIs(t, err, testData.expected)
		})
	}
}
