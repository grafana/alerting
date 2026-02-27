package definition

import (
	"encoding/json"
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
