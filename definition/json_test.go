package definition

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/config"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/timeinterval"
	commoncfg "github.com/prometheus/common/config"

	httpcfg "github.com/grafana/alerting/http/v0mimir1"
	"github.com/grafana/alerting/receivers"
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
							URL: &receivers.SecretURL{URL: testURL},
							HTTPConfig: &httpcfg.HTTPClientConfig{
								BasicAuth: &httpcfg.BasicAuth{
									Username: "user",
									Password: commoncfg.Secret("password"),
								},
							},
						},
						{
							URL: &receivers.SecretURL{URL: testURL},
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
							AuthPassword: receivers.Secret("smtp-password"),
							AuthSecret:   receivers.Secret("smtp-secret"),
							Headers:      map[string]string{},
							HTML:         "{{ template \"email.default.html\" . }}",
						},
						{
							To:           "test2@grafana.com",
							From:         "alerts2@grafana.com",
							AuthUsername: "smtp-user2",
							AuthPassword: receivers.Secret(""),
							AuthSecret:   receivers.Secret("smtp-secret2"),
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
			secret:         receivers.Secret("my-secret"),
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
			secret:         receivers.Secret(""),
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
			secret:         receivers.Secret("secret with spaces\nand\t 🔑"),
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
			secretURL:      &receivers.SecretURL{URL: testURL},
			expectStandard: maskedSecret,
			expectPlain:    fmt.Sprintf(`"%s"`, u),
		},
		{
			name:           "pointer to empty URL",
			secretURL:      &receivers.SecretURL{},
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
		// receivers types
		RSecret    receivers.Secret     `json:"r_secret,omitempty"`
		RSecretPtr *receivers.Secret    `json:"r_secret_ptr,omitempty"`
		RURL       receivers.SecretURL  `json:"r_url,omitempty"`
		RURLPtr    *receivers.SecretURL `json:"r_url_ptr,omitempty"`
		// config types
		CSecret    config.Secret     `json:"c_secret,omitempty"`
		CSecretPtr *config.Secret    `json:"c_secret_ptr,omitempty"`
		CURL       config.SecretURL  `json:"c_url,omitempty"`
		CURLPtr    *config.SecretURL `json:"c_url_ptr,omitempty"`
		// common config types
		CCSecret    commoncfg.Secret  `json:"cc_secret,omitempty"`
		CCSecretPtr *commoncfg.Secret `json:"cc_secret_ptr,omitempty"`
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
				RSecret:     receivers.Secret("rs1"),
				RSecretPtr:  func() *receivers.Secret { s := receivers.Secret("rs2"); return &s }(),
				RURL:        receivers.SecretURL{URL: &url.URL{Scheme: "https", Host: "r.example.com"}},
				RURLPtr:     &receivers.SecretURL{URL: &url.URL{Scheme: "https", Host: "r2.example.com"}},
				CSecret:     config.Secret("cs1"),
				CSecretPtr:  func() *config.Secret { s := config.Secret("cs2"); return &s }(),
				CURL:        config.SecretURL{URL: &url.URL{Scheme: "https", Host: "c.example.com"}},
				CURLPtr:     &config.SecretURL{URL: &url.URL{Scheme: "https", Host: "c2.example.com"}},
				CCSecret:    commoncfg.Secret("ccs1"),
				CCSecretPtr: func() *commoncfg.Secret { s := commoncfg.Secret("ccs2"); return &s }(),
			},
			expected: `{
				"r_secret": "rs1",
				"r_secret_ptr": "rs2",
				"r_url": "https://r.example.com",
				"r_url_ptr": "https://r2.example.com",
				"c_secret": "cs1",
				"c_secret_ptr": "cs2",
				"c_url": "https://c.example.com",
				"c_url_ptr": "https://c2.example.com",
				"cc_secret": "ccs1",
				"cc_secret_ptr": "ccs2"
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
