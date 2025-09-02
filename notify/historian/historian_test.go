package historian

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"testing"

	"time"

	"github.com/go-kit/log"
	alertingInstrument "github.com/grafana/alerting/http/instrument"
	"github.com/grafana/alerting/http/instrument/instrumenttest"
	alertingModels "github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/historian/lokiclient"
	"github.com/grafana/alerting/notify/nfstatus"
	"github.com/grafana/dskit/instrument"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
)

const testReceiverName = "testReceiverName"

var (
	testGroupLabels  = model.LabelSet{"foo": "bar"}
	testPipelineTime = time.Date(2025, time.July, 15, 16, 55, 0, 0, time.UTC)
	testNow          = time.Now()
	testAlerts       = []*types.Alert{
		{
			Alert: model.Alert{
				Labels:       model.LabelSet{"alertname": "Alert1", alertingModels.RuleUIDLabel: "testRuleUID"},
				Annotations:  model.LabelSet{"foo": "bar", "__private__": "baz"},
				StartsAt:     testPipelineTime,
				EndsAt:       testPipelineTime,
				GeneratorURL: "http://localhost/test",
			},
		},
	}
)

func TestRecord(t *testing.T) {
	t.Run("write notification history to Loki", func(t *testing.T) {
		testCases := []struct {
			name            string
			retry           bool
			notificationErr error
			expected        []lokiclient.Stream
		}{
			{
				"successful notification",
				false,
				nil,
				[]lokiclient.Stream{
					{
						Stream: map[string]string{
							"externalLabelKey": "externalLabelValue",
							"from":             "notify-history",
							"ruleUID":          "testRuleUID",
						},
						Values: []lokiclient.Sample{
							{
								T: testNow,
								V: "{\"schemaVersion\":1,\"receiver\":\"testReceiverName\",\"status\":\"resolved\",\"groupLabels\":{\"foo\":\"bar\"},\"alerts\":[{\"status\":\"resolved\",\"labels\":{\"__alert_rule_uid__\":\"testRuleUID\",\"alertname\":\"Alert1\"},\"annotations\":{\"__private__\":\"baz\",\"foo\":\"bar\"},\"startsAt\":\"2025-07-15T16:55:00Z\",\"endsAt\":\"2025-07-15T16:55:00Z\"}],\"retry\":false,\"duration\":1000,\"pipelineTime\":\"2025-07-15T16:55:00Z\"}",
							},
						},
					},
				},
			},
			{
				"failed notification",
				true,
				errors.New("test notification error"),
				[]lokiclient.Stream{
					{
						Stream: map[string]string{
							"externalLabelKey": "externalLabelValue",
							"from":             "notify-history",
							"ruleUID":          "testRuleUID",
						},
						Values: []lokiclient.Sample{
							{
								T: testNow,
								V: "{\"schemaVersion\":1,\"receiver\":\"testReceiverName\",\"status\":\"resolved\",\"groupLabels\":{\"foo\":\"bar\"},\"alerts\":[{\"status\":\"resolved\",\"labels\":{\"__alert_rule_uid__\":\"testRuleUID\",\"alertname\":\"Alert1\"},\"annotations\":{\"__private__\":\"baz\",\"foo\":\"bar\"},\"startsAt\":\"2025-07-15T16:55:00Z\",\"endsAt\":\"2025-07-15T16:55:00Z\"}],\"retry\":true,\"error\":\"test notification error\",\"duration\":1000,\"pipelineTime\":\"2025-07-15T16:55:00Z\"}",
							},
						},
					},
				},
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := instrumenttest.NewFakeRequester()
				writesTotal := prometheus.NewCounter(prometheus.CounterOpts{})
				writesFailed := prometheus.NewCounter(prometheus.CounterOpts{})

				h := createTestNotificationHistorian(req, writesTotal, writesFailed)
				h.Record(context.Background(), nfstatus.NotificationHistoryEntry{
					Alerts:          testAlerts,
					Retry:           tc.retry,
					NotificationErr: tc.notificationErr,
					Duration:        time.Second,
					ReceiverName:    testReceiverName,
					GroupLabels:     testGroupLabels,
					PipelineTime:    testPipelineTime,
				})

				reqBody, err := io.ReadAll(req.LastRequest.Body)
				require.NoError(t, err)

				type LokiRequestBody struct {
					Streams []lokiclient.Stream `json:"streams"`
				}
				var lrb LokiRequestBody
				err = json.Unmarshal(reqBody, &lrb)
				require.NoError(t, err)

				for i := range lrb.Streams {
					for j := range lrb.Streams[i].Values {
						// Overwrite the timestamp to make the test deterministic.
						lrb.Streams[i].Values[j].T = testNow
					}
				}
				require.Equal(t, tc.expected, lrb.Streams)
			})
		}
	})

	t.Run("emits expected write metrics", func(t *testing.T) {
		writesTotal := prometheus.NewCounter(prometheus.CounterOpts{})
		writesFailed := prometheus.NewCounter(prometheus.CounterOpts{})

		goodHistorian := createTestNotificationHistorian(instrumenttest.NewFakeRequester(), writesTotal, writesFailed)
		badHistorian := createTestNotificationHistorian(instrumenttest.NewFakeRequester().WithResponse(instrumenttest.BadResponse()), writesTotal, writesFailed)

		nhe := nfstatus.NotificationHistoryEntry{
			Alerts:          testAlerts,
			Retry:           false,
			NotificationErr: nil,
			Duration:        time.Second,
			ReceiverName:    testReceiverName,
			GroupLabels:     testGroupLabels,
			PipelineTime:    testPipelineTime,
		}
		goodHistorian.Record(context.Background(), nhe)
		badHistorian.Record(context.Background(), nhe)

		require.Equal(t, 2, int(testutil.ToFloat64(writesTotal)))
		require.Equal(t, 1, int(testutil.ToFloat64(writesFailed)))
	})
}

func createTestNotificationHistorian(req alertingInstrument.Requester, writesTotal prometheus.Counter, writesFailed prometheus.Counter) *NotificationHistorian {
	writePathURL, _ := url.Parse("http://some.url")
	cfg := lokiclient.LokiConfig{
		WritePathURL:   writePathURL,
		ExternalLabels: map[string]string{"externalLabelKey": "externalLabelValue"},
		Encoder:        lokiclient.JSONEncoder{},
	}

	bytesWritten := prometheus.NewCounter(prometheus.CounterOpts{})
	writeDuration := instrument.NewHistogramCollector(prometheus.NewHistogramVec(prometheus.HistogramOpts{}, instrument.HistogramCollectorBuckets))

	return NewNotificationHistorian(log.NewNopLogger(), cfg, req, bytesWritten, writeDuration, writesTotal, writesFailed, noop.NewTracerProvider().Tracer("test"))
}
