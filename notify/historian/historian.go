package historian

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/instrument"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	prometheusModel "github.com/prometheus/common/model"
	"go.opentelemetry.io/otel/trace"

	alertingInstrument "github.com/grafana/alerting/http/instrument"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/historian/lokiclient"
	"github.com/grafana/alerting/notify/nfstatus"
)

const (
	LokiClientSpanName              = "ngalert.notification-historian.client"
	NotificationHistoryWriteTimeout = time.Minute
	LabelFrom                       = "from"
	LabelFromValue                  = "notify-history"
	LabelFromValueAlerts            = "notify-history-alerts"
	SchemaVersion                   = 2
)

type NotificationHistoryLokiEntry struct {
	SchemaVersion  int               `json:"schemaVersion"`
	UUID           string            `json:"uuid"`
	RuleUIDs       []string          `json:"ruleUIDs"`
	Receiver       string            `json:"receiver"`
	Integration    string            `json:"integration"`
	IntegrationIdx int               `json:"integrationIdx"`
	GroupKey       string            `json:"groupKey"`
	Status         string            `json:"status"`
	GroupLabels    map[string]string `json:"groupLabels"`
	AlertCount     int               `json:"alertCount"`
	Retry          bool              `json:"retry"`
	Error          string            `json:"error,omitempty"`
	Duration       int64             `json:"duration"`
	PipelineTime   time.Time         `json:"pipelineTime"`
}

type NotificationHistoryLokiEntryAlert struct {
	SchemaVersion int               `json:"schemaVersion"`
	UUID          string            `json:"uuid"`
	AlertIndex    int               `json:"alertIndex"`
	Status        string            `json:"status"`
	Labels        map[string]string `json:"labels"`
	Annotations   map[string]string `json:"annotations"`
	StartsAt      time.Time         `json:"startsAt"`
	EndsAt        time.Time         `json:"endsAt"`
	ExtraData     json.RawMessage   `json:"enrichments,omitempty"`
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
	req alertingInstrument.Requester,
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

func (h *NotificationHistorian) Record(ctx context.Context, nhe nfstatus.NotificationHistoryEntry) {
	streams, err := h.prepareStreams(nhe)
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

// prepareStreams prepares the data to be written to Loki. It is written to two streams:
// 1. Contains a log line per notification, and contains metadata about the notification as a whole.
// 2. Contains a log line per alert per notification, and a UUID linking back to the notification.
func (h *NotificationHistorian) prepareStreams(nhe nfstatus.NotificationHistoryEntry) ([]lokiclient.Stream, error) {
	now := time.Now()
	alertsValues := make([]lokiclient.Sample, len(nhe.Alerts))
	ruleUIDsMap := make(map[string]struct{})
	for i, alert := range nhe.Alerts {
		labels := prepareLabels(alert.Labels)
		annotations := prepareLabels(alert.Annotations)
		entryAlert := NotificationHistoryLokiEntryAlert{
			SchemaVersion: SchemaVersion,
			UUID:          nhe.UUID,
			AlertIndex:    i,
			Labels:        labels,
			Annotations:   annotations,
			Status:        string(alert.StatusAt(now)),
			StartsAt:      alert.StartsAt,
			EndsAt:        alert.EndsAt,
			ExtraData:     alert.ExtraData,
		}

		entryAlertJSON, err := json.Marshal(entryAlert)
		if err != nil {
			return []lokiclient.Stream{}, fmt.Errorf("marshal alert entry: %w", err)
		}

		// Loki pagination is done with timestamps, and notifications can have many alerts.
		// Therefore, to be able to return notifications with > 5000 alerts, we should give
		// each line a slightly different timestamp.
		ts := now.Add(time.Nanosecond * time.Duration(i))

		ruleUID := entryAlert.Labels[models.RuleUIDLabel]
		alertsValues[i] = lokiclient.Sample{
			T: ts,
			V: string(entryAlertJSON),
			Metadata: map[string]string{
				"uuid":     nhe.UUID,
				"rule_uid": ruleUID,
			},
		}

		ruleUIDsMap[ruleUID] = struct{}{}
	}

	notificationErrStr := ""
	if nhe.NotificationErr != nil {
		notificationErrStr = nhe.NotificationErr.Error()
	}

	as := make([]*types.Alert, len(nhe.Alerts))
	for i := range nhe.Alerts {
		as[i] = nhe.Alerts[i].Alert
	}

	ruleUIDs := slices.Sorted(maps.Keys(ruleUIDsMap))

	entry := NotificationHistoryLokiEntry{
		SchemaVersion:  SchemaVersion,
		UUID:           nhe.UUID,
		RuleUIDs:       ruleUIDs,
		Receiver:       nhe.ReceiverName,
		Integration:    nhe.IntegrationName,
		IntegrationIdx: nhe.IntegrationIdx,
		Status:         string(types.Alerts(as...).StatusAt(now)),
		GroupKey:       nhe.GroupKey,
		GroupLabels:    prepareLabels(nhe.GroupLabels),
		AlertCount:     len(nhe.Alerts),
		Retry:          nhe.Retry,
		Error:          notificationErrStr,
		Duration:       int64(nhe.Duration),
		PipelineTime:   nhe.PipelineTime,
	}

	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return []lokiclient.Stream{}, fmt.Errorf("marshal notification entry: %w", err)
	}

	// Even though there are potentially multiple rule UIDs per notification, it is still
	// beneficial to store them as structured metadata, as we can perform a regex match
	// on the field to avoids Loki having to parse the entire log line.
	entryValue := lokiclient.Sample{
		T: now,
		V: string(entryJSON),
		Metadata: map[string]string{
			"uuid":      nhe.UUID,
			"receiver":  nhe.ReceiverName,
			"rule_uids": strings.Join(ruleUIDs, ","),
		},
	}

	streamLabels := make(map[string]string)
	streamLabels[LabelFrom] = LabelFromValue
	for k, v := range h.externalLabels {
		streamLabels[k] = v
	}

	alertsStreamLabels := make(map[string]string)
	alertsStreamLabels[LabelFrom] = LabelFromValueAlerts
	for k, v := range h.externalLabels {
		alertsStreamLabels[k] = v
	}

	return []lokiclient.Stream{
		// The notification history entry itself.
		{
			Stream: streamLabels,
			Values: []lokiclient.Sample{entryValue},
		},
		// The individual alert details entries.
		{
			Stream: alertsStreamLabels,
			Values: alertsValues,
		},
	}, nil
}

func prepareLabels(labels prometheusModel.LabelSet) map[string]string {
	result := make(map[string]string, len(labels))
	for k, v := range labels {
		result[string(k)] = string(v)
	}
	return result
}
