package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestNotify(t *testing.T) {
	imageProvider := images.NewFakeProvider(1)
	singleURLConfig := Config{
		URLs: []*url.URL{
			receiversTesting.ParseURLUnsafe("https://alertmanager.com/api/v1/alerts"),
		},
		User:     "admin",
		Password: "password",
	}

	cases := []struct {
		name                 string
		settings             Config
		alerts               []*types.Alert
		expectedError        string
		sendHTTPRequestError error
	}{
		{
			name:     "Default config with one alert",
			settings: singleURLConfig,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
		}, {
			name:     "Default config with one alert with empty receiver name",
			settings: singleURLConfig,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
		}, {
			name:     "Error sending to Alertmanager",
			settings: singleURLConfig,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
			},
			expectedError:        "failed to send alert to Alertmanager: expected error",
			sendHTTPRequestError: errors.New("expected error"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sn := &Notifier{
				Base:     receivers.NewBase(receivers.Metadata{}, log.NewNopLogger()),
				images:   imageProvider,
				settings: c.settings,
			}

			var body []byte
			origSendHTTPRequest := receivers.SendHTTPRequest
			t.Cleanup(func() {
				receivers.SendHTTPRequest = origSendHTTPRequest
			})
			receivers.SendHTTPRequest = func(_ context.Context, _ *url.URL, cfg receivers.HTTPCfg, _ log.Logger) ([]byte, error) {
				body = cfg.Body
				assert.Equal(t, c.settings.User, cfg.User)
				assert.Equal(t, c.settings.Password, cfg.Password)
				return nil, c.sendHTTPRequestError
			}

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
			ok, err := sn.Notify(ctx, c.alerts...)

			if c.sendHTTPRequestError != nil {
				require.EqualError(t, err, c.expectedError)
				require.False(t, ok)
			} else {
				require.NoError(t, err)
				require.True(t, ok)
				expBody, err := json.Marshal(c.alerts)
				require.NoError(t, err)
				require.JSONEq(t, string(expBody), string(body))
			}
		})
	}

	t.Run("images should not modify the original alerts", func(t *testing.T) {
		getAlerts := func() []*types.Alert {
			return []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1"},
						Annotations: model.LabelSet{"__alertImageToken__": "test-image-1"},
					},
				},
			}
		}

		sn := &Notifier{
			Base:     receivers.NewBase(receivers.Metadata{}, log.NewNopLogger()),
			images:   imageProvider,
			settings: singleURLConfig,
		}
		alerts := getAlerts()

		origSendHTTPRequest := receivers.SendHTTPRequest
		t.Cleanup(func() {
			receivers.SendHTTPRequest = origSendHTTPRequest
		})
		var body []byte
		receivers.SendHTTPRequest = func(_ context.Context, _ *url.URL, cfg receivers.HTTPCfg, _ log.Logger) ([]byte, error) {
			body = cfg.Body
			return nil, nil
		}

		ctx := notify.WithGroupKey(context.Background(), "alertname")
		ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})

		_, err := sn.Notify(ctx, alerts...)
		require.NoError(t, err)
		require.EqualValues(t, getAlerts(), alerts)
		expectedAlerts := getAlerts()
		expectedAlerts[0].Annotations["image"] = "https://www.example.com/test-image-1.jpg"
		expectedBody, err := json.Marshal(expectedAlerts)
		require.NoError(t, err)
		require.JSONEq(t, string(expectedBody), string(body))
	})

	t.Run("preserves generator URL and timestamps", func(t *testing.T) {
		start := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Millisecond)
		end := time.Now().Add(10 * time.Minute).UTC().Truncate(time.Millisecond)
		genURL := "https://example.com/gen/abc123"
		alerts := []*types.Alert{
			{
				Alert: model.Alert{
					Labels:       model.LabelSet{"alertname": "timed"},
					Annotations:  model.LabelSet{"note": "has times"},
					StartsAt:     start,
					EndsAt:       end,
					GeneratorURL: genURL,
				},
			},
		}
		orig := receivers.SendHTTPRequest
		t.Cleanup(func() { receivers.SendHTTPRequest = orig })
		var body []byte
		receivers.SendHTTPRequest = func(_ context.Context, _ *url.URL, cfg receivers.HTTPCfg, _ log.Logger) ([]byte, error) {
			body = cfg.Body
			return nil, nil
		}
		sn := &Notifier{Base: receivers.NewBase(receivers.Metadata{}, log.NewNopLogger()), images: imageProvider, settings: singleURLConfig}
		ok, err := sn.Notify(context.Background(), alerts...)
		require.NoError(t, err)
		require.True(t, ok)
		var sent []*types.Alert
		require.NoError(t, json.Unmarshal(body, &sent))
		require.Len(t, sent, 1)
		require.Equal(t, start, sent[0].StartsAt)
		require.Equal(t, end, sent[0].EndsAt)
		require.Equal(t, genURL, sent[0].GeneratorURL)
	})
}
