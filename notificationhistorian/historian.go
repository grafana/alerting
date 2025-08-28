package notificationhistorian

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alerting/client"
	"github.com/grafana/alerting/lokiclient"
	alertingModels "github.com/grafana/alerting/models"
	"github.com/grafana/dskit/instrument"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	prometheusModel "github.com/prometheus/common/model"
	"go.opentelemetry.io/otel/trace"
)

const (
	LokiClientSpanName              = "ngalert.notification-historian.client"
	NotificationHistoryWriteTimeout = time.Minute
	LabelFrom                       = "from"
	LabelFromValue                  = "notify-history"
	LabelRuleUID                    = "ruleUID"
)

type NotificationHistoryLokiEntry struct {
	SchemaVersion int                                 `json:"schemaVersion"`
	Receiver      string                              `json:"receiver"`
	Status        string                              `json:"status"`
	GroupLabels   map[string]string                   `json:"groupLabels"`
	Alerts        []NotificationHistoryLokiEntryAlert `json:"alerts"`
	Retry         bool                                `json:"retry"`
	Error         string                              `json:"error,omitempty"`
	Duration      int64                               `json:"duration"`
	PipelineTime  time.Time                           `json:"pipelineTime"`
}

type NotificationHistoryLokiEntryAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
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

func (h *NotificationHistorian) Record(
	ctx context.Context,
	alerts []*types.Alert,
	retry bool,
	notificationErr error,
	duration time.Duration,
	receiverName string,
	groupLabels prometheusModel.LabelSet,
	pipelineTime time.Time,
	now time.Time,
) {
	streams, err := h.prepareStreams(alerts, retry, notificationErr, duration, receiverName, groupLabels, pipelineTime, now)
	if err != nil {
		level.Error(h.logger).Log("msg", "Failed to convert notification history to streams", "error", err)
		return
	}

	// This is a new background job, so let's create a new context for it.
	// We want it to be isolated, i.e. we don't want grafana shutdowns to interrupt this work
	// immediately but rather try to flush writes.
	// This also prevents timeouts or other lingering objects (like transactions) from being
	// incorrectly propagated here from other areas.
	writeCtx, cancel := context.WithTimeout(context.Background(), NotificationHistoryWriteTimeout)
	writeCtx = trace.ContextWithSpan(writeCtx, trace.SpanFromContext(ctx))
	defer cancel()

	level.Debug(h.logger).Log("msg", "Saving notification history")
	h.writesTotal.Inc()

	if err := h.client.Push(writeCtx, streams); err != nil {
		level.Error(h.logger).Log("msg", "Failed to save notification history", "error", err)
		h.writesFailed.Inc()
	}
	level.Debug(h.logger).Log("msg", "Done saving notification history")
}

func (h *NotificationHistorian) prepareStreams(
	alerts []*types.Alert,
	retry bool,
	notificationErr error,
	duration time.Duration,
	receiverName string,
	groupLabels prometheusModel.LabelSet,
	pipelineTime time.Time,
	now time.Time,
) ([]lokiclient.Stream, error) {
	// group alerts by rule UID. each rule UID will be a separate stream.
	ruleUIDToAlerts := make(map[prometheusModel.LabelValue][]*types.Alert)
	for _, alert := range alerts {
		ruleUID, ok := alert.Labels[alertingModels.RuleUIDLabel]
		if !ok {
			return []lokiclient.Stream{}, fmt.Errorf("rule UID not found in labels")
		}
		ruleUIDToAlerts[ruleUID] = append(ruleUIDToAlerts[ruleUID], alert)
	}

	streams := make([]lokiclient.Stream, 0)
	for ruleUID := range ruleUIDToAlerts {
		stream, err := h.prepareStream(ruleUIDToAlerts[ruleUID], retry, notificationErr, duration, receiverName, groupLabels, pipelineTime, now)
		if err != nil {
			return []lokiclient.Stream{}, err
		}
		streams = append(streams, stream)
	}

	return streams, nil
}

func (h *NotificationHistorian) prepareStream(
	alerts []*types.Alert,
	retry bool,
	notificationErr error,
	duration time.Duration,
	receiverName string,
	groupLabels prometheusModel.LabelSet,
	pipelineTime time.Time,
	now time.Time,
) (lokiclient.Stream, error) {
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
		PipelineTime:  pipelineTime,
	}

	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return lokiclient.Stream{}, err
	}

	streamLabels := make(map[string]string)
	streamLabels[LabelFrom] = LabelFromValue
	streamLabels[LabelRuleUID] = string(alerts[0].Labels[alertingModels.RuleUIDLabel])

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

func prepareLabels(labels prometheusModel.LabelSet) map[string]string {
	result := make(map[string]string)
	for k, v := range labels {
		result[string(k)] = string(v)
	}
	return result
}
