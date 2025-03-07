package stages

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/nflog/nflogpb"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsTimeWithinMargin(t *testing.T) {
	stage := &SyncFlushStage{
		margin: 2 * time.Second,
	}

	now := time.Now()
	tests := []struct {
		name       string
		entryTime  time.Time
		nextFlush  time.Time
		expectTrue bool
	}{
		{
			name:       "nextFlush before entryTime",
			entryTime:  now,
			nextFlush:  now.Add(-1 * time.Second),
			expectTrue: true, // Returns true when nextFlush is before entryTime
		},
		{
			name:       "nextFlush within margin",
			entryTime:  now,
			nextFlush:  now.Add(1 * time.Second),
			expectTrue: false, // Returns false when within margin
		},
		{
			name:       "nextFlush at margin boundary",
			entryTime:  now,
			nextFlush:  now.Add(2 * time.Second),
			expectTrue: false, // Returns false at exact margin boundary
		},
		{
			name:       "nextFlush after margin",
			entryTime:  now,
			nextFlush:  now.Add(3 * time.Second),
			expectTrue: true, // Returns true when after margin
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := stage.isTimeWithinMargin(tc.entryTime, tc.nextFlush)
			assert.Equal(t, tc.expectTrue, result)
		})
	}
}

func TestSyncFlushStageExec(t *testing.T) {
	now := time.Now()
	groupKey := "test-group"
	groupWait := 30 * time.Second

	tests := []struct {
		name           string
		sync           bool
		entries        []*nflogpb.Entry
		pipelineTime   time.Time
		contextTimeout time.Duration
		expectedWait   bool
		expectedErr    bool
	}{
		{
			name:         "no entries",
			sync:         true,
			entries:      []*nflogpb.Entry{},
			pipelineTime: now,
			expectedWait: false,
			expectedErr:  false,
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
			expectedWait: false,
			expectedErr:  false,
		},
		{
			name: "entry exists and wait needed",
			sync: true,
			entries: []*nflogpb.Entry{
				{
					GroupKey:     groupKey,
					PipelineTime: now.Add(-10 * time.Second),
				},
			},
			pipelineTime: now,
			expectedWait: true,
			expectedErr:  false,
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
			expectedWait: false, // No wait when sync is disabled
			expectedErr:  false,
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
			expectedWait:   true,
			expectedErr:    true,
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
			expectedWait: false,
			expectedErr:  true,
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

			// Setup context with required values
			ctx = notify.WithGroupKey(ctx, groupKey)
			ctx = notify.WithNow(ctx, tc.pipelineTime)
			ctx = notify.WithGroupWait(ctx, groupWait)

			// Create mock notification log
			nflog := &testNflog{
				qres: tc.entries,
			}

			// Create stage
			stage := &SyncFlushStage{
				nflog:  nflog,
				recv:   &nflogpb.Receiver{GroupName: "test-receiver", Integration: "test-integration"},
				sync:   tc.sync,
				margin: 2 * time.Second,
			}

			alerts := []*types.Alert{{}, {}}
			startTime := time.Now()
			_, gotAlerts, err := stage.Exec(ctx, log.NewNopLogger(), alerts...)
			execDuration := time.Since(startTime)

			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, alerts, gotAlerts)
			}

			// Verify wait behavior
			if tc.expectedWait && tc.sync && !tc.expectedErr {
				// Should have waited some time, but not too long
				assert.Greater(t, execDuration, 50*time.Millisecond)
			} else if !tc.expectedWait && !tc.expectedErr {
				// Should have returned quickly
				assert.Less(t, execDuration, 50*time.Millisecond)
			}
		})
	}
}

func TestNewSyncFlushStage(t *testing.T) {
	tests := []struct {
		name          string
		action        SyncFlushAction
		expectNil     bool
		expectedSync  bool
		expectedNflog bool
		expectedRecv  bool
	}{
		{
			name:          "log action",
			action:        SyncFlushActionLog,
			expectNil:     false,
			expectedSync:  false,
			expectedNflog: true,
			expectedRecv:  true,
		},
		{
			name:          "sync action",
			action:        SyncFlushActionSync,
			expectNil:     false,
			expectedSync:  true,
			expectedNflog: true,
			expectedRecv:  true,
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

			stage := NewSyncFlushStage(nflog, recv, tc.action)

			if tc.expectNil {
				assert.Nil(t, stage)
			} else {
				assert.NotNil(t, stage)
				assert.Equal(t, tc.expectedSync, stage.sync)
				if tc.expectedNflog {
					assert.Equal(t, nflog, stage.nflog)
				}
				if tc.expectedRecv {
					assert.Equal(t, recv, stage.recv)
				}
				assert.Equal(t, 2*time.Second, stage.margin)
			}
		})
	}
}
