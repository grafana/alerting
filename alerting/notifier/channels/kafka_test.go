package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestKafkaNotifier(t *testing.T) {
	tmpl := templateForTests(t)

	images := newFakeImageStore(2)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	cases := []struct {
		name           string
		settings       string
		alerts         []*types.Alert
		expURL, expMsg string
		expInitError   string
		expMsgError    error
		expUsername    string
		expPassword    string
		expHTTPHeader  map[string]string
	}{
		{
			name: "A single alert with image and custom description and details",
			settings: `{
				"kafkaRestProxy": "http://localhost",
				"kafkaTopic": "sometopic",
				"description": "customDescription",
				"details": "customDetails"
			}`,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expURL: "http://localhost/topics/sometopic",
			expMsg: `{
				  "records": [
					{
					  "value": {
						"alert_state": "alerting",
						"client": "Grafana",
						"client_url": "http://localhost/alerting/list",
						"contexts": [{"type": "image", "src": "https://www.example.com/test-image-1.jpg"}],
						"description": "customDescription",
						"details": "customDetails",
						"incident_key": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733"
					  }
					}
				  ]
				}`,
			expMsgError: nil,
			expUsername: "",
			expPassword: "",
			expHTTPHeader: map[string]string{
				"Content-Type": "application/vnd.kafka.json.v2+json",
				"Accept":       "application/vnd.kafka.v2+json",
			},
		}, {
			name: "A single alert with image and custom description and details with auth",
			settings: `{
				"kafkaRestProxy": "http://localhost",
				"kafkaTopic": "sometopic",
				"description": "customDescription",
				"details": "customDetails",
				"username": "batman",
				"password": "BruceWayne"
			}`,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expURL: "http://localhost/topics/sometopic",
			expMsg: `{
				  "records": [
					{
					  "value": {
						"alert_state": "alerting",
						"client": "Grafana",
						"client_url": "http://localhost/alerting/list",
						"contexts": [{"type": "image", "src": "https://www.example.com/test-image-1.jpg"}],
						"description": "customDescription",
						"details": "customDetails",
						"incident_key": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733"
					  }
					}
				  ]
				}`,
			expMsgError: nil,
			expUsername: "batman",
			expPassword: "BruceWayne",
			expHTTPHeader: map[string]string{
				"Content-Type": "application/vnd.kafka.json.v2+json",
				"Accept":       "application/vnd.kafka.v2+json",
			},
		}, {
			name: "Multiple alerts with images with default description and details",
			settings: `{
				"kafkaRestProxy": "http://localhost",
				"kafkaTopic": "sometopic"
			}`,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__alertImageToken__": "test-image-1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2", "__alertImageToken__": "test-image-2"},
					},
				},
			},
			expURL: "http://localhost/topics/sometopic",
			expMsg: `{
				  "records": [
					{
					  "value": {
						"alert_state": "alerting",
						"client": "Grafana",
						"client_url": "http://localhost/alerting/list",
						"contexts": [{"type": "image", "src": "https://www.example.com/test-image-1.jpg"}, {"type": "image", "src": "https://www.example.com/test-image-2.jpg"}],
						"description": "[FIRING:2]  ",
						"details": "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
						"incident_key": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733"
					  }
					}
				  ]
				}`,
			expMsgError: nil,
		}, {
			name:         "Endpoint missing",
			settings:     `{"kafkaTopic": "sometopic"}`,
			expInitError: `could not find kafka rest proxy endpoint property in settings`,
		}, {
			name:         "Topic missing",
			settings:     `{"kafkaRestProxy": "http://localhost"}`,
			expInitError: `could not find kafka topic property in settings`,
		}, {
			name: "Bad API version",
			settings: `
			{
				"kafkaRestProxy": "http://localhost",
				"kafkaTopic": "myTopic",
				"apiVersion": "invalid"
			}
			`,
			expInitError: `unsupported api version: invalid`,
		}, {
			name: "API v3. Missing cluster ID",
			settings: `
			{
				"kafkaRestProxy": "http://localhost",
				"kafkaTopic": "myTopic",
				"apiVersion": "v3"
			}
			`,
			expInitError: `kafka cluster id must be provided when using api version 3`,
		}, {
			name: "API v3 verify URL, description and details",
			settings: `
			{
				"kafkaRestProxy": "http://localhost:882",
				"kafkaTopic": "myTopic",
				"apiVersion": "v3",
				"kafkaClusterId": "lkc-abcd"
			}
			`,
			expURL:      `http://localhost:882/kafka/v3/clusters/lkc-abcd/topics/myTopic/records`,
			expMsgError: nil,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expMsg: `
			{
				"value": {
					"type": "JSON",
					"data": {
						"alert_state": "alerting",
						"client": "Grafana",
						"client_url": "http://localhost/alerting/list",
						"contexts": [{"type": "image", "src": "https://www.example.com/test-image-1.jpg"}],
						"description":"[FIRING:1]  (val1)",
						"details":"**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
						"incident_key": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733"
					}
				}
			}`,
		}, {
			name: "API v3 verify single alert with image and custom description and details",
			settings: `
			{
				"kafkaRestProxy": "http://localhost:882",
				"kafkaTopic": "myTopic",
				"apiVersion": "v3",
				"kafkaClusterId": "lkc-abcd",
				"description": "customDescription",
				"details": "customDetails"
			}
			`,
			expURL:      `http://localhost:882/kafka/v3/clusters/lkc-abcd/topics/myTopic/records`,
			expMsgError: nil,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expMsg: `
			{
				"value": {
					"type": "JSON",
					"data": {
						"alert_state": "alerting",
						"client": "Grafana",
						"client_url": "http://localhost/alerting/list",
						"contexts": [{"type": "image", "src": "https://www.example.com/test-image-1.jpg"}],
						"description": "customDescription",
						"details": "customDetails",
						"incident_key": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733"
					}
				}
			}`,
		},
		{
			name: "API v3 multiple alerts with images with default description and details",
			settings: `
			{
				"kafkaRestProxy": "http://localhost:882",
				"kafkaTopic": "myTopic",
				"apiVersion": "v3",
				"kafkaClusterId": "lkc-abcd"
			}
			`,
			expURL:      `http://localhost:882/kafka/v3/clusters/lkc-abcd/topics/myTopic/records`,
			expMsgError: nil,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__alertImageToken__": "test-image-1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2", "__alertImageToken__": "test-image-2"},
					},
				},
			},
			expMsg: `
			{
				"value": {
					"type": "JSON",
					"data": {
						"alert_state": "alerting",
						"client": "Grafana",
						"client_url": "http://localhost/alerting/list",
						"contexts": [{"type": "image", "src": "https://www.example.com/test-image-1.jpg"}, {"type": "image", "src": "https://www.example.com/test-image-2.jpg"}],
						"description": "[FIRING:2]  ",
						"details": "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
						"incident_key": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733"
					}
				}
			}`,
			expHTTPHeader: map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
			},
		},
		{
			name: "API v3 multiple alerts with images with default description and details and auth",
			settings: `
			{
				"kafkaRestProxy": "http://localhost:882",
				"kafkaTopic": "myTopic",
				"apiVersion": "v3",
				"kafkaClusterId": "lkc-abcd",
				"username": "batman",
				"password": "BruceWayne"
			}
			`,
			expURL:      `http://localhost:882/kafka/v3/clusters/lkc-abcd/topics/myTopic/records`,
			expMsgError: nil,
			expUsername: "batman",
			expPassword: "BruceWayne",
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__alertImageToken__": "test-image-1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2", "__alertImageToken__": "test-image-2"},
					},
				},
			},
			expMsg: `
			{
				"value": {
					"type": "JSON",
					"data": {
						"alert_state": "alerting",
						"client": "Grafana",
						"client_url": "http://localhost/alerting/list",
						"contexts": [{"type": "image", "src": "https://www.example.com/test-image-1.jpg"}, {"type": "image", "src": "https://www.example.com/test-image-2.jpg"}],
						"description": "[FIRING:2]  ",
						"details": "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
						"incident_key": "6e3538104c14b583da237e9693b76debbc17f0f8058ef20492e5853096cf8733"
					}
				}
			}`,
			expHTTPHeader: map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			webhookSender := mockNotificationService()

			fc := FactoryConfig{
				Config: &NotificationChannelConfig{
					Name:     "kafka_testing",
					Type:     "kafka",
					Settings: json.RawMessage(c.settings),
				},
				ImageStore: images,
				// TODO: allow changing the associated values for different tests.
				NotificationService: webhookSender,
				DecryptFunc: func(ctx context.Context, sjd map[string][]byte, key string, fallback string) string {
					return fallback
				},
				Template: tmpl,
				Logger:   &FakeLogger{},
			}

			pn, err := newKafkaNotifier(fc)
			if c.expInitError != "" {
				require.Error(t, err)
				require.Equal(t, c.expInitError, err.Error())
				return
			}
			require.NoError(t, err)

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})

			ok, err := pn.Notify(ctx, c.alerts...)
			if c.expMsgError != nil {
				require.False(t, ok)
				require.Error(t, err)
				require.Equal(t, c.expMsgError.Error(), err.Error())
				return
			}
			require.NoError(t, err)
			require.True(t, ok)

			require.Equal(t, c.expURL, webhookSender.Webhook.URL)
			require.JSONEq(t, c.expMsg, webhookSender.Webhook.Body)
			require.Equal(t, c.expUsername, webhookSender.Webhook.User)
			require.Equal(t, c.expPassword, webhookSender.Webhook.Password)
			if c.expHTTPHeader != nil {
				// As of go 1.12 maps are printed in key-sorted order to ease testing
				// Ref: https://tip.golang.org/doc/go1.12#fmt
				require.Equal(t, fmt.Sprint(c.expHTTPHeader), fmt.Sprint(webhookSender.Webhook.HTTPHeader))
			}
		})
	}
}
