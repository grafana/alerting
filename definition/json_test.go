package definition

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/timeinterval"
	commoncfg "github.com/prometheus/common/config"

	httpcfg "github.com/grafana/alerting/http/v0mimir1"
	email_v0mimir1 "github.com/grafana/alerting/receivers/email/v0mimir1"
	webhook_v0mimir1 "github.com/grafana/alerting/receivers/webhook/v0mimir1"
)

func TestMarshalJSONWithSecrets(t *testing.T) {
	u := "https://grafana.com/webhook"
	testURL, err := url.Parse(u)
	require.NoError(t, err)

	amsLoc, err := time.LoadLocation("Europe/Amsterdam")
	require.NoError(t, err)

	// stdlib json escapes < and > characters,
	// so just marshal the placeholder string to have the same value.
	maskedSecretBytes, err := json.Marshal("<secret>")
	require.NoError(t, err)
	maskedSecret := string(maskedSecretBytes)

	cfg := PostableApiAlertingConfig{
		Config: Config{
			Route: &Route{
				Receiver: "test-receiver",
			},
			TimeIntervals: []config.TimeInterval{
				{
					Name: "time-interval-1",
					TimeIntervals: []timeinterval.TimeInterval{
						{
							Times: []timeinterval.TimeRange{
								{
									StartMinute: 60,
									EndMinute:   120,
								},
							},
							Weekdays: []timeinterval.WeekdayRange{
								{
									InclusiveRange: timeinterval.InclusiveRange{
										Begin: 1,
										End:   5,
									},
								},
							},
						},
					},
				},
				{
					Name: "time-interval-2",
					TimeIntervals: []timeinterval.TimeInterval{
						{
							Times: []timeinterval.TimeRange{
								{
									StartMinute: 120,
									EndMinute:   240,
								},
							},
							Weekdays: []timeinterval.WeekdayRange{
								{
									InclusiveRange: timeinterval.InclusiveRange{
										Begin: 0,
										End:   2,
									},
								},
							},
							Location: &timeinterval.Location{Location: amsLoc},
						},
					},
				},
			},
		},
		Receivers: []*PostableApiReceiver{
			{
				Receiver: Receiver{
					Name: "test-receiver",
					WebhookConfigs: []*webhook_v0mimir1.Config{
						{
							URL: &config.SecretURL{URL: testURL},
							HTTPConfig: &httpcfg.HTTPClientConfig{
								BasicAuth: &httpcfg.BasicAuth{
									Username: "user",
									Password: commoncfg.Secret("password"),
								},
							},
						},
						{
							URL: &config.SecretURL{URL: testURL},
							HTTPConfig: &httpcfg.HTTPClientConfig{
								Authorization: &httpcfg.Authorization{
									Type:        "Bearer",
									Credentials: commoncfg.Secret("bearer-token-secret"),
								},
							},
						},
					},
					EmailConfigs: []*email_v0mimir1.Config{
						{
							To:           "test@grafana.com",
							From:         "alerts@grafana.com",
							AuthUsername: "smtp-user",
							AuthPassword: config.Secret("smtp-password"),
							AuthSecret:   config.Secret("smtp-secret"),
							Headers:      map[string]string{},
							HTML:         "{{ template \"email.default.html\" . }}",
						},
						{
							To:           "test2@grafana.com",
							From:         "alerts2@grafana.com",
							AuthUsername: "smtp-user2",
							AuthPassword: config.Secret(""),
							AuthSecret:   config.Secret("smtp-secret2"),
							Headers:      map[string]string{},
							HTML:         "{{ template \"email.default.html\" . }}",
						},
					},
				},
			},
		},
	}

	standardJSON, err := json.Marshal(cfg)
	require.NoError(t, err)

	plainJSONBytes, err := MarshalJSONWithSecrets(cfg)
	require.NoError(t, err)
	require.True(t, json.Valid(plainJSONBytes))

	require.True(t, json.Valid(standardJSON))
	require.Contains(t, string(standardJSON), maskedSecret)

	var roundTripCfg PostableApiAlertingConfig
	err = json.Unmarshal(plainJSONBytes, &roundTripCfg)
	require.NoError(t, err)
	require.Equal(t, cfg, roundTripCfg)
}

func TestSecretTypeMarshaling(t *testing.T) {
	// stdlib json escapes < and > characters,
	// so just marshal the placeholder string to have the same value.
	maskedSecretBytes, err := json.Marshal("<secret>")
	require.NoError(t, err)
	maskedSecret := string(maskedSecretBytes)

	tests := []struct {
		name           string
		secret         any
		expectStandard string
		expectPlain    string
	}{
		{
			name:           "nil",
			secret:         nil,
			expectStandard: `null`,
			expectPlain:    `null`,
		},
		{
			name:           "alertmanager config secret",
			secret:         config.Secret("my-secret"),
			expectStandard: maskedSecret,
			expectPlain:    `"my-secret"`,
		},
		{
			name:           "common config secret",
			secret:         commoncfg.Secret("common-secret"),
			expectStandard: maskedSecret,
			expectPlain:    `"common-secret"`,
		},
		{
			name:           "empty alertmanager secret",
			secret:         config.Secret(""),
			expectStandard: maskedSecret,
			expectPlain:    `""`,
		},
		{
			name:           "empty common secret",
			secret:         commoncfg.Secret(""),
			expectStandard: `""`,
			expectPlain:    `""`,
		},
		{
			name:           "nil alertmanager secret pointer",
			secret:         (*config.Secret)(nil),
			expectStandard: "null",
			expectPlain:    "null",
		},
		{
			name:           "nil common config secret pointer",
			secret:         (*commoncfg.Secret)(nil),
			expectStandard: "null",
			expectPlain:    "null",
		},
		{
			name:           "pointer to alertmanager secret",
			secret:         func() *config.Secret { s := config.Secret("pointer-secret"); return &s }(),
			expectStandard: maskedSecret,
			expectPlain:    `"pointer-secret"`,
		},
		{
			name:           "pointer to common secret",
			secret:         func() *commoncfg.Secret { s := commoncfg.Secret("pointer-common"); return &s }(),
			expectStandard: maskedSecret,
			expectPlain:    `"pointer-common"`,
		},
		{
			name:           "secret with special characters",
			secret:         config.Secret("secret with spaces\nand\t 🔑"),
			expectStandard: maskedSecret,
			expectPlain:    `"secret with spaces\nand\t 🔑"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			standard, err := json.Marshal(tt.secret)
			require.NoError(t, err)
			require.Equal(t, tt.expectStandard, string(standard))

			plain, err := MarshalJSONWithSecrets(tt.secret)
			require.NoError(t, err)
			require.Equal(t, tt.expectPlain, string(plain))
		})
	}
}

func TestSecretURLTypeMarshaling(t *testing.T) {
	u := "https://grafana.com/webhook"
	testURL, err := url.Parse(u)
	require.NoError(t, err)

	// stdlib json escapes < and > characters,
	// so just marshal the placeholder string to have the same value.
	maskedSecretBytes, err := json.Marshal("<secret>")
	require.NoError(t, err)
	maskedSecret := string(maskedSecretBytes)

	complexURL, err := url.Parse("https://user:pass@example.com:8080/path?query=value#fragment")
	require.NoError(t, err)

	tests := []struct {
		name           string
		secretURL      interface{}
		expectStandard string
		expectPlain    string
	}{
		{
			name:           "non-empty URL",
			secretURL:      config.SecretURL{URL: testURL},
			expectStandard: maskedSecret,
			expectPlain:    fmt.Sprintf(`"%s"`, u),
		},
		{
			name:           "empty URL",
			secretURL:      config.SecretURL{},
			expectStandard: maskedSecret,
			expectPlain:    `null`,
		},
		{
			name:           "complex URL",
			secretURL:      config.SecretURL{URL: complexURL},
			expectStandard: maskedSecret,
			expectPlain:    fmt.Sprintf(`"%s"`, complexURL.String()),
		},
		{
			name:           "nil URL pointer",
			secretURL:      (*config.SecretURL)(nil),
			expectStandard: "null",
			expectPlain:    "null",
		},
		{
			name:           "URL pointer",
			secretURL:      &config.SecretURL{URL: testURL},
			expectStandard: maskedSecret,
			expectPlain:    fmt.Sprintf(`"%s"`, u),
		},
		{
			name:           "pointer to empty URL",
			secretURL:      &config.SecretURL{},
			expectStandard: maskedSecret,
			expectPlain:    `null`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			standard, err := json.Marshal(tt.secretURL)
			require.NoError(t, err)
			require.Equal(t, tt.expectStandard, string(standard))

			plain, err := MarshalJSONWithSecrets(tt.secretURL)
			require.NoError(t, err)
			require.Equal(t, tt.expectPlain, string(plain))
		})
	}
}

func TestSecretOmitempty(t *testing.T) {
	type testStruct struct {
		Secret    config.Secret     `json:"secret,omitempty"`
		SecretPtr *config.Secret    `json:"secret_ptr,omitempty"`
		URL       config.SecretURL  `json:"url,omitempty"`
		URLPtr    *config.SecretURL `json:"url_ptr,omitempty"`
	}

	tests := []struct {
		name     string
		value    testStruct
		expected string
	}{
		{
			name:     "all empty",
			value:    testStruct{},
			expected: `{}`,
		},
		{
			name: "all present",
			value: testStruct{
				Secret:    config.Secret("secret1"),
				SecretPtr: func() *config.Secret { s := config.Secret("secret2"); return &s }(),
				URL:       config.SecretURL{URL: &url.URL{Scheme: "https", Host: "example.com"}},
				URLPtr:    &config.SecretURL{URL: &url.URL{Scheme: "https", Host: "example2.com"}},
			},
			expected: `{
				"secret": "secret1",
				"secret_ptr": "secret2",
				"url": "https://example.com",
				"url_ptr": "https://example2.com"
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalJSONWithSecrets(tt.value)
			require.NoError(t, err)
			require.JSONEq(t, tt.expected, string(result))
		})
	}
}
