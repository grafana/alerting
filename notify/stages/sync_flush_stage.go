package stages

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/alertmanager/nflog"
	"github.com/prometheus/alertmanager/nflog/nflogpb"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
)

type SyncFlushAction string

const (
	SyncFlushActionDisabled SyncFlushAction = "disabled"
	SyncFlushActionLog      SyncFlushAction = "log"
	SyncFlushActionSync     SyncFlushAction = "sync"
)

var (
	ErrMissingGroupKey           = errors.New("groupKey missing")
	ErrMissingNow                = errors.New("now missing")
	ErrMissingGroupInterval      = errors.New("groupInterval missing")
	ErrUnexpectedEntryResultSize = errors.New("unexpected entry result size")
)

// SyncFlushStage delays the notification pipeline execution to sync flushes between multiple instances.
type SyncFlushStage struct {
	nflog  notify.NotificationLog
	recv   *nflogpb.Receiver
	sync   bool
	margin time.Duration
}

// NewSyncFlushStage creates a new SyncFlushStage.
func NewSyncFlushStage(l notify.NotificationLog, recv *nflogpb.Receiver, action SyncFlushAction, margin time.Duration) *SyncFlushStage {
	var sync bool
	switch action {
	case SyncFlushActionLog:
		sync = false
	case SyncFlushActionSync:
		sync = true
	default:
		return nil
	}
	return &SyncFlushStage{
		nflog:  l,
		recv:   recv,
		sync:   sync,
		margin: margin,
	}
}

// calculateSyncWaitTime calculates the wait time needed to synchronize flush operations.
func (sfs *SyncFlushStage) calculateSyncWaitTime(curPipelineTime, prevPipelineTime time.Time, groupInterval time.Duration) (wait time.Duration) {
	nextFlush := prevPipelineTime.Add(groupInterval)

	// NOTE: if nextFlush is before curPipelineTime, don't try to sync the flush time
	if nextFlush.Before(curPipelineTime) {
		return
	}

	// if diff is greater than margin, we should wait
	if diff := nextFlush.Sub(curPipelineTime); diff > sfs.margin {
		wait = diff
	}
	return
}

// Exec implements the Stage interface.
func (sfs *SyncFlushStage) Exec(ctx context.Context, l log.Logger, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	gkey, ok := notify.GroupKey(ctx)
	if !ok {
		return ctx, nil, ErrMissingGroupKey
	}

	// get the tick time from the context.
	curPipelineTime, ok := notify.Now(ctx)
	if !ok {
		return ctx, nil, ErrMissingNow
	}

	groupInterval, ok := notify.GroupInterval(ctx)
	if !ok {
		return ctx, nil, ErrMissingGroupInterval
	}

	entries, err := sfs.nflog.Query(nflog.QGroupKey(gkey), nflog.QReceiver(sfs.recv))
	if err != nil && !errors.Is(err, nflog.ErrNotFound) {
		_ = level.Debug(l).Log("msg", "error querying log entry", "error", err, "pipeline_time", curPipelineTime, "aggrGroup", gkey, "alerts", fmt.Sprintf("%+v", alerts), "receiver", sfs.recv.GroupName, "integration", sfs.recv.Integration)
		return ctx, nil, err
	}

	var entry *nflogpb.Entry
	switch len(entries) {
	case 0:
		return ctx, alerts, nil
	case 1:
		entry = entries[0]
	default:
		return ctx, nil, fmt.Errorf("%w: %d", ErrUnexpectedEntryResultSize, len(entries))
	}

	// calculate next flush time based on last entry on notification log
	wait := sfs.calculateSyncWaitTime(curPipelineTime, entry.PipelineTime, groupInterval)
	_ = level.Debug(l).Log(
		"msg", "syncing flush time",
		"pipeline_time", curPipelineTime,
		"aggrGroup", gkey,
		"alerts", fmt.Sprintf("%+v", alerts),
		"receiver", sfs.recv.GroupName,
		"integration", sfs.recv.Integration,
		"entry_pipeline_time", entry.PipelineTime,
		"wait", wait,
		"sync", sfs.sync,
	)

	if sfs.sync {
		select {
		case <-time.After(wait):
			ctx = notify.WithNow(ctx, curPipelineTime.Add(wait))
		case <-ctx.Done():
			return ctx, nil, ctx.Err()
		}
	}

	return ctx, alerts, nil
}
