package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alerting/client"
	"github.com/grafana/alerting/lokiclient"
	alertingModels "github.com/grafana/alerting/models"
	"github.com/grafana/dskit/instrument"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	prometheusModel "github.com/prometheus/common/model"
	"go.opentelemetry.io/otel/trace"
)

const LokiClientSpanName = "ngalert.notification-historian.client"
const NotificationHistoryWriteTimeout = time.Minute
const NotificationHistoryKey = "from"
const NotificationHistoryLabelValue = "notify-history"

type NotificationHistoryLokiEntry struct {
	SchemaVersion int                                 `json:"schemaVersion"`
	Receiver      string                              `json:"receiver"`
	Status        string                              `json:"status"`
	GroupLabels   map[string]string                   `json:"groupLabels"`
	Alerts        []NotificationHistoryLokiEntryAlert `json:"alerts"`
	Retry         bool                                `json:"retry"`
	Error         string                              `json:"error,omitempty"`
	Duration      int64                               `json:"duration"`
}

type NotificationHistoryLokiEntryAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
	RuleUID     string            `json:"ruleUID"`
}

type remoteLokiClient interface {
	Ping(context.Context) error
	Push(context.Context, []lokiclient.Stream) error
}

type NotificationHistorian struct {
	client         remoteLokiClient
	externalLabels map[string]string
	writesTotal    prometheus.Counter
	writesFailed   prometheus.Counter
	logger         log.Logger
}

func NewNotificationHistorian(
	logger log.Logger,
	cfg lokiclient.LokiConfig,
	req client.Requester,
	bytesWritten prometheus.Counter,
	writeDuration *instrument.HistogramCollector,
	writesTotal prometheus.Counter,
	writesFailed prometheus.Counter,
	tracer trace.Tracer,
) *NotificationHistorian {
	return &NotificationHistorian{
		client:         lokiclient.NewLokiClient(cfg, req, bytesWritten, writeDuration, logger, tracer, LokiClientSpanName),
		externalLabels: cfg.ExternalLabels,
		writesTotal:    writesTotal,
		writesFailed:   writesFailed,
		logger:         logger,
	}
}

func (h *NotificationHistorian) TestConnection(ctx context.Context) error {
	return h.client.Ping(ctx)
}

func (h *NotificationHistorian) Record(ctx context.Context, alerts []*types.Alert, retry bool, notificationErr error, duration time.Duration) <-chan error {
	stream, err := h.prepareStream(ctx, alerts, retry, notificationErr, duration)
	errCh := make(chan error, 1)
	if err != nil {
		level.Error(h.logger).Log("msg", "Failed to convert notification history to stream", "error", err)
		errCh <- fmt.Errorf("failed to convert notification history to stream: %w", err)
		close(errCh)
		return errCh
	}

	// This is a new background job, so let's create a new context for it.
	// We want it to be isolated, i.e. we don't want grafana shutdowns to interrupt this work
	// immediately but rather try to flush writes.
	// This also prevents timeouts or other lingering objects (like transactions) from being
	// incorrectly propagated here from other areas.
	writeCtx := context.Background()
	writeCtx, cancel := context.WithTimeout(writeCtx, NotificationHistoryWriteTimeout)
	writeCtx = trace.ContextWithSpan(writeCtx, trace.SpanFromContext(ctx))

	go func(ctx context.Context) {
		defer cancel()
		defer close(errCh)
		level.Debug(h.logger).Log("msg", "Saving notification history")
		h.writesTotal.Inc()

		if err := h.recordStream(ctx, stream, h.logger); err != nil {
			level.Error(h.logger).Log("msg", "Failed to save notification history", "error", err)
			h.writesFailed.Inc()
			errCh <- fmt.Errorf("failed to save notification history: %w", err)
		}
	}(writeCtx)
	return errCh
}

func (h *NotificationHistorian) prepareStream(ctx context.Context, alerts []*types.Alert, retry bool, notificationErr error, duration time.Duration) (lokiclient.Stream, error) {
	receiverName, ok := notify.ReceiverName(ctx)
	if !ok {
		return lokiclient.Stream{}, fmt.Errorf("receiver name not found in context")
	}
	groupLabels, ok := notify.GroupLabels(ctx)
	if !ok {
		return lokiclient.Stream{}, fmt.Errorf("group labels not found in context")
	}
	now, ok := notify.Now(ctx)
	if !ok {
		return lokiclient.Stream{}, fmt.Errorf("now not found in context")
	}

	entryAlerts := make([]NotificationHistoryLokiEntryAlert, len(alerts))
	for i, alert := range alerts {
		labels := prepareLabels(alert.Labels)
		annotations := prepareLabels(alert.Annotations)
		entryAlerts[i] = NotificationHistoryLokiEntryAlert{
			Labels:      labels,
			Annotations: annotations,
			Status:      string(alert.StatusAt(now)),
			StartsAt:    alert.StartsAt,
			EndsAt:      alert.EndsAt,
			RuleUID:     string(alert.Labels[alertingModels.RuleUIDLabel]),
		}
	}

	notificationErrStr := ""
	if notificationErr != nil {
		notificationErrStr = notificationErr.Error()
	}

	entry := NotificationHistoryLokiEntry{
		SchemaVersion: 1,
		Receiver:      receiverName,
		Status:        string(types.Alerts(alerts...).StatusAt(now)),
		GroupLabels:   prepareLabels(groupLabels),
		Alerts:        entryAlerts,
		Retry:         retry,
		Error:         notificationErrStr,
		Duration:      duration.Milliseconds(),
	}

	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return lokiclient.Stream{}, err
	}

	streamLabels := make(map[string]string)
	streamLabels[NotificationHistoryKey] = NotificationHistoryLabelValue
	for k, v := range h.externalLabels {
		streamLabels[k] = v
	}

	return lokiclient.Stream{
		Stream: streamLabels,
		Values: []lokiclient.Sample{
			{
				T: now,
				V: string(entryJSON),
			}},
	}, nil
}

func (h *NotificationHistorian) recordStream(ctx context.Context, stream lokiclient.Stream, logger log.Logger) error {
	if err := h.client.Push(ctx, []lokiclient.Stream{stream}); err != nil {
		return err
	}
	level.Debug(logger).Log("msg", "Done saving notification history")
	return nil
}

func prepareLabels(labels prometheusModel.LabelSet) map[string]string {
	result := make(map[string]string)
	for k, v := range labels {
		// Remove private labels
		if !strings.HasPrefix(string(k), "__") && !strings.HasSuffix(string(k), "__") {
			result[string(k)] = string(v)
		}
	}
	return result
}
