// copy of https://github.com/grafana/mimir/blob/a5f6bc75e858f2f7ede4b68bd692ed9b4f99193d/pkg/alertmanager/api_test.go

package definition

import (
	"testing"

	"github.com/prometheus/alertmanager/config"
	commoncfg "github.com/prometheus/common/config"
	"github.com/stretchr/testify/assert"

	httpcfg "github.com/grafana/alerting/http/v0mimir1"
	discord_v0mimir1 "github.com/grafana/alerting/receivers/discord/v0mimir1"
	email_v0mimir1 "github.com/grafana/alerting/receivers/email/v0mimir1"
	teams_v0mimir1 "github.com/grafana/alerting/receivers/teams/v0mimir1"
)

func TestValidateAlertmanagerConfig(t *testing.T) {
	tests := map[string]struct {
		input    interface{}
		expected error
	}{
		"*HTTPClientConfig": {
			input: &httpcfg.HTTPClientConfig{
				BasicAuth: &httpcfg.BasicAuth{
					PasswordFile: "/secrets",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"HTTPClientConfig": {
			input: httpcfg.HTTPClientConfig{
				BasicAuth: &httpcfg.BasicAuth{
					PasswordFile: "/secrets",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"*TLSConfig": {
			input: &httpcfg.TLSConfig{
				CertFile: "/cert",
			},
			expected: errTLSConfigNotAllowed,
		},
		"TLSConfig": {
			input: httpcfg.TLSConfig{
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
			input: &discord_v0mimir1.Config{
				HTTPConfig: &httpcfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"DiscordConfig.HTTPConfig": {
			input: discord_v0mimir1.Config{
				HTTPConfig: &httpcfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"*DiscordConfig.WebhookURLFile": {
			input: &discord_v0mimir1.Config{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"DiscordConfig.WebhookURLFile": {
			input: discord_v0mimir1.Config{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"*EmailConfig.AuthPasswordFile": {
			input: &email_v0mimir1.Config{
				AuthPasswordFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"EmailConfig.AuthPasswordFile": {
			input: email_v0mimir1.Config{
				AuthPasswordFile: "/file",
			},
			expected: errPasswordFileNotAllowed,
		},
		"*MSTeams.HTTPConfig": {
			input: &teams_v0mimir1.Config{
				HTTPConfig: &httpcfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"MSTeams.HTTPConfig": {
			input: teams_v0mimir1.Config{
				HTTPConfig: &httpcfg.HTTPClientConfig{
					BearerTokenFile: "/file",
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"*MSTeams.WebhookURLFile": {
			input: &teams_v0mimir1.Config{
				WebhookURLFile: "/file",
			},
			expected: errWebhookURLFileNotAllowed,
		},
		"MSTeams.WebhookURLFile": {
			input: teams_v0mimir1.Config{
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
		"map containing *HTTPClientConfig": {
			input: map[string]*httpcfg.HTTPClientConfig{
				"test": {
					BasicAuth: &httpcfg.BasicAuth{
						PasswordFile: "/secrets",
					},
				},
			},
			expected: errPasswordFileNotAllowed,
		},
		"map containing TLSConfig as nested child": {
			input: map[string][]email_v0mimir1.Config{
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
