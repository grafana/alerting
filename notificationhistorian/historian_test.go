package notificationhistorian

import (
	"context"
	"errors"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alerting/client"
	"github.com/grafana/alerting/lokiclient"
	alertingModels "github.com/grafana/alerting/models"
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
	testNow          = time.Date(2025, time.August, 28, 13, 15, 0, 0, time.UTC)
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
			expected        string
		}{
			{
				"successful notification",
				false,
				nil,
				"{\"streams\":[{\"stream\":{\"externalLabelKey\":\"externalLabelValue\",\"from\":\"notify-history\",\"ruleUID\":\"testRuleUID\"},\"values\":[[\"1756386900000000000\",\"{\\\"schemaVersion\\\":1,\\\"receiver\\\":\\\"testReceiverName\\\",\\\"status\\\":\\\"resolved\\\",\\\"groupLabels\\\":{\\\"foo\\\":\\\"bar\\\"},\\\"alerts\\\":[{\\\"status\\\":\\\"resolved\\\",\\\"labels\\\":{\\\"__alert_rule_uid__\\\":\\\"testRuleUID\\\",\\\"alertname\\\":\\\"Alert1\\\"},\\\"annotations\\\":{\\\"__private__\\\":\\\"baz\\\",\\\"foo\\\":\\\"bar\\\"},\\\"startsAt\\\":\\\"2025-07-15T16:55:00Z\\\",\\\"endsAt\\\":\\\"2025-07-15T16:55:00Z\\\"}],\\\"retry\\\":false,\\\"duration\\\":1000,\\\"pipelineTime\\\":\\\"2025-07-15T16:55:00Z\\\"}\"]]}]}",
			},
			{
				"failed notification",
				true,
				errors.New("test notification error"),
				"{\"streams\":[{\"stream\":{\"externalLabelKey\":\"externalLabelValue\",\"from\":\"notify-history\",\"ruleUID\":\"testRuleUID\"},\"values\":[[\"1756386900000000000\",\"{\\\"schemaVersion\\\":1,\\\"receiver\\\":\\\"testReceiverName\\\",\\\"status\\\":\\\"resolved\\\",\\\"groupLabels\\\":{\\\"foo\\\":\\\"bar\\\"},\\\"alerts\\\":[{\\\"status\\\":\\\"resolved\\\",\\\"labels\\\":{\\\"__alert_rule_uid__\\\":\\\"testRuleUID\\\",\\\"alertname\\\":\\\"Alert1\\\"},\\\"annotations\\\":{\\\"__private__\\\":\\\"baz\\\",\\\"foo\\\":\\\"bar\\\"},\\\"startsAt\\\":\\\"2025-07-15T16:55:00Z\\\",\\\"endsAt\\\":\\\"2025-07-15T16:55:00Z\\\"}],\\\"retry\\\":true,\\\"error\\\":\\\"test notification error\\\",\\\"duration\\\":1000,\\\"pipelineTime\\\":\\\"2025-07-15T16:55:00Z\\\"}\"]]}]}",
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := lokiclient.NewFakeRequester()
				writesTotal := prometheus.NewCounter(prometheus.CounterOpts{})
				writesFailed := prometheus.NewCounter(prometheus.CounterOpts{})

				// TODO: check metrics updated
				h := createTestNotificationHistorian(req, writesTotal, writesFailed)

				h.Record(context.Background(), testAlerts, tc.retry, tc.notificationErr, time.Second, testReceiverName, testGroupLabels, testPipelineTime, testNow)

				reqBody, err := io.ReadAll(req.LastRequest.Body)
				require.NoError(t, err)
				require.Equal(t, tc.expected, string(reqBody))
			})
		}
	})

	t.Run("emits expected write metrics", func(t *testing.T) {
		// TODO: fix mess with metrics
		writesTotal := prometheus.NewCounter(prometheus.CounterOpts{})
		writesFailed := prometheus.NewCounter(prometheus.CounterOpts{})

		goodHistorian := createTestNotificationHistorian(lokiclient.NewFakeRequester(), writesTotal, writesFailed)
		badHistorian := createTestNotificationHistorian(lokiclient.NewFakeRequester().WithResponse(lokiclient.BadResponse()), writesTotal, writesFailed)

		goodHistorian.Record(context.Background(), testAlerts, false, nil, time.Second, testReceiverName, testGroupLabels, testPipelineTime, testNow)
		badHistorian.Record(context.Background(), testAlerts, false, nil, time.Second, testReceiverName, testGroupLabels, testPipelineTime, testNow)

		require.Equal(t, 2, int(testutil.ToFloat64(writesTotal)))
		require.Equal(t, 1, int(testutil.ToFloat64(writesFailed)))
	})
}

func createTestNotificationHistorian(req client.Requester, writesTotal prometheus.Counter, writesFailed prometheus.Counter) *NotificationHistorian {
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
