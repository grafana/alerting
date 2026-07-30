package definition

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/config"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/timeinterval"
)

func TestMarshalJSONWithSecrets(t *testing.T) {
	slackAPIURL := "https://grafana.com/slack-webhook"
	testURL, err := url.Parse(slackAPIURL)
	require.NoError(t, err)

	amsLoc, err := time.LoadLocation("Europe/Amsterdam")
	require.NoError(t, err)

	// stdlib json escapes < and > characters,
	// so just marshal the placeholder string to have the same value.
	maskedSecretBytes, err := json.Marshal("<secret>")
	require.NoError(t, err)
	maskedSecret := string(maskedSecretBytes)

	globalConfig := config.DefaultGlobalConfig()
	globalConfig.SMTPAuthPassword = config.Secret("smtp-password")
	globalConfig.SlackAPIURL = (*config.SecretURL)(&config.URL{URL: testURL})

	cfg := PostableApiAlertingConfig{
		Config: Config{
			Global: &globalConfig,
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
				Name: "test-receiver",
				PostableGrafanaReceivers: PostableGrafanaReceivers{
					GrafanaManagedReceivers: []*PostableGrafanaReceiver{
						{
							UID:  "test-uid",
							Name: "test-receiver",
							Type: "slack",
							SecureSettings: map[string]string{
								"url": "https://grafana.com/slack-webhook",
							},
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
