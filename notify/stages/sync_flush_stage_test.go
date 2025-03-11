package stages

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/nflog/nflogpb"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/stretchr/testify/assert"
)

func TestCalculateSyncWaitTime(t *testing.T) {
	stage := &SyncFlushStage{
		margin: 2 * time.Second,
	}

	now := time.Now()
	groupWait := 30 * time.Second
	tests := []struct {
		name             string
		curPipelineTime  time.Time
		prevPipelineTime time.Time
		groupWait        time.Duration
		expectedWait     time.Duration
	}{
		{
			name:             "nextFlush before curPipelineTime",
			curPipelineTime:  now,
			prevPipelineTime: now.Add(-31 * time.Second), // nextFlush = prevPipelineTime + groupWait = now - 1s
			groupWait:        groupWait,
			expectedWait:     0,
		},
		{
			name:             "nextFlush within margin",
			curPipelineTime:  now,
			prevPipelineTime: now.Add(-29 * time.Second), // nextFlush = prevPipelineTime + groupWait = now + 1s
			groupWait:        groupWait,
			expectedWait:     0,
		},
		{
			name:             "nextFlush at margin boundary",
			curPipelineTime:  now,
			prevPipelineTime: now.Add(-28 * time.Second), // nextFlush = prevPipelineTime + groupWait = now + 2s
			groupWait:        groupWait,
			expectedWait:     0,
		},
		{
			name:             "nextFlush after margin",
			curPipelineTime:  now,
			prevPipelineTime: now.Add(-27 * time.Second), // nextFlush = prevPipelineTime + groupWait = now + 3s
			groupWait:        groupWait,
			expectedWait:     3 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := stage.calculateSyncWaitTime(tc.curPipelineTime, tc.prevPipelineTime, tc.groupWait)
			assert.Equal(t, tc.expectedWait, result)
		})
	}
}

func TestSyncFlushStageExec(t *testing.T) {
	now := time.Now()
	groupKey := []byte("test-group")
	groupWait := 30 * time.Second
	nflogQueryErr := errors.New("nflog query error")

	tests := []struct {
		name           string
		sync           bool
		entries        []*nflogpb.Entry
		pipelineTime   time.Time
		contextTimeout time.Duration
		skipGroupKey   bool
		skipNow        bool
		skipGroupWait  bool
		nflogErr       error
		expectedErr    error
	}{
		{
			name:         "no entries",
			sync:         true,
			entries:      []*nflogpb.Entry{},
			pipelineTime: now,
		},
		{
			name:         "missing group key",
			sync:         true,
			entries:      []*nflogpb.Entry{},
			pipelineTime: now,
			skipGroupKey: true,
			expectedErr:  ErrMissingGroupKey,
		},
		{
			name:         "missing now",
			sync:         true,
			entries:      []*nflogpb.Entry{},
			pipelineTime: now,
			skipNow:      true,
			expectedErr:  ErrMissingNow,
		},
		{
			name:          "missing group wait",
			sync:          true,
			entries:       []*nflogpb.Entry{},
			pipelineTime:  now,
			skipGroupWait: true,
			expectedErr:   ErrMissingGroupInterval,
		},
		{
			name:         "nflog query error",
			pipelineTime: now,
			nflogErr:     nflogQueryErr,
			expectedErr:  nflogQueryErr,
		},
		{
			name: "entry exists but no wait needed",
			sync: true,
			entries: []*nflogpb.Entry{
				{
					GroupKey:     groupKey,
					PipelineTime: now.Add(-groupWait),
				},
			},
			pipelineTime: now,
		},
		{
			name: "entry exists and wait would be needed",
			sync: true,
			entries: []*nflogpb.Entry{
				{
					GroupKey:     groupKey,
					PipelineTime: now.Add(-10 * time.Second),
				},
			},
			pipelineTime: now,
		},
		{
			name: "sync disabled",
			sync: false,
			entries: []*nflogpb.Entry{
				{
					GroupKey:     groupKey,
					PipelineTime: now.Add(-10 * time.Second),
				},
			},
			pipelineTime: now,
		},
		{
			name: "context timeout",
			sync: true,
			entries: []*nflogpb.Entry{
				{
					GroupKey:     groupKey,
					PipelineTime: now.Add(-10 * time.Second),
				},
			},
			pipelineTime:   now,
			contextTimeout: 50 * time.Millisecond,
			expectedErr:    context.DeadlineExceeded,
		},
		{
			name: "multiple entries error",
			sync: true,
			entries: []*nflogpb.Entry{
				{
					GroupKey:     groupKey,
					PipelineTime: now.Add(-10 * time.Second),
				},
				{
					GroupKey:     groupKey,
					PipelineTime: now.Add(-5 * time.Second),
				},
			},
			pipelineTime: now,
			expectedErr:  ErrUnexpectedEntryResultSize,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.contextTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.contextTimeout)
				defer cancel()
			}

			if !tc.skipGroupKey {
				ctx = notify.WithGroupKey(ctx, string(groupKey))
			}
			if !tc.skipNow {
				ctx = notify.WithNow(ctx, tc.pipelineTime)
			}
			if !tc.skipGroupWait {
				ctx = notify.WithGroupInterval(ctx, groupWait)
			}

			nflog := &testNflog{
				qerr: tc.nflogErr,
				qres: tc.entries,
			}

			stage := &SyncFlushStage{
				nflog:  nflog,
				recv:   &nflogpb.Receiver{GroupName: "test-receiver", Integration: "test-integration"},
				sync:   tc.sync,
				margin: 2 * time.Second,
			}

			_, _, err := stage.Exec(ctx, log.NewNopLogger(), &types.Alert{})
			assert.ErrorIs(t, err, tc.expectedErr)
		})
	}
}

func TestNewSyncFlushStage(t *testing.T) {
	tests := []struct {
		name         string
		action       SyncFlushAction
		expectNil    bool
		expectedSync bool
	}{
		{
			name:         "log action",
			action:       SyncFlushActionLog,
			expectNil:    false,
			expectedSync: false,
		},
		{
			name:         "sync action",
			action:       SyncFlushActionSync,
			expectNil:    false,
			expectedSync: true,
		},
		{
			name:      "disabled action",
			action:    SyncFlushActionDisabled,
			expectNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nflog := &testNflog{}
			recv := &nflogpb.Receiver{}

			stage := NewSyncFlushStage(nflog, recv, tc.action, time.Second)

			if tc.expectNil {
				assert.Nil(t, stage)
			} else {
				assert.NotNil(t, stage)
				assert.Equal(t, tc.expectedSync, stage.sync)
			}
		})
	}
}
