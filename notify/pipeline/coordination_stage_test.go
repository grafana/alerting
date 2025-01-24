package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/nflog"
	"github.com/prometheus/alertmanager/nflog/nflogpb"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineAndStateTimestampCoordinationStage(t *testing.T) {
	t0 := time.Now()
	tplus := t0.Add(time.Second)
	tminus := t0.Add(-time.Second)

	state := func(tc time.Time) *nflogpb.Entry {
		return &nflogpb.Entry{FiringAlerts: []uint64{0, 1, 2}, Timestamp: tc}
	}

	testCases := []struct {
		desc              string
		state             *nflogpb.Entry
		stateError        error
		pipelineTimeStamp time.Time
		stopPipeline      bool
		expectedStop      bool
	}{
		{
			desc:              "should not stop if state < pipeline",
			state:             state(t0),
			pipelineTimeStamp: tplus,
			expectedStop:      false,
		},
		{
			desc:              "should not stop if state = pipeline",
			state:             state(t0),
			pipelineTimeStamp: t0,
			expectedStop:      false,
		},
		{
			desc:              "should stop if state > pipeline and stopPipeline=true",
			state:             state(t0),
			pipelineTimeStamp: tminus,
			stopPipeline:      true,
			expectedStop:      true,
		},
		{
			desc:              "should not stop if state > pipeline and stopPipeline=false",
			state:             state(t0),
			pipelineTimeStamp: tminus,
			stopPipeline:      false,
			expectedStop:      false,
		},
		{
			desc:              "should not stop if no state",
			state:             nil,
			stateError:        nflog.ErrNotFound,
			pipelineTimeStamp: tminus,
			stopPipeline:      true,
			expectedStop:      false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			n := &testNflog{}
			if tc.state != nil {
				n.qres = append(n.qres, tc.state)
			}
			if tc.stateError != nil {
				n.qerr = tc.stateError
			}
			ctx := notify.WithGroupKey(notify.WithNow(context.Background(), tc.pipelineTimeStamp), "test-group")
			stage := PipelineAndStateTimestampCoordinationStage{
				nflog:        n,
				stopPipeline: tc.stopPipeline,
			}
			alerts := []*types.Alert{{}, {}, {}}
			rCtx, rAlerts, err := stage.Exec(ctx, log.NewNopLogger(), alerts...)
			require.NoError(t, err)
			assert.Equal(t, ctx, rCtx)
			if tc.expectedStop {
				require.Empty(t, rAlerts)
			} else {
				require.Equal(t, alerts, rAlerts)
			}
		})
	}
}

type testNflog struct {
	qres []*nflogpb.Entry
	qerr error

	logFunc func(r *nflogpb.Receiver, gkey string, firingAlerts, resolvedAlerts []uint64, expiry time.Duration) error
}

func (l *testNflog) Query(_ ...nflog.QueryParam) ([]*nflogpb.Entry, error) {
	return l.qres, l.qerr
}

func (l *testNflog) Log(r *nflogpb.Receiver, gkey string, firingAlerts, resolvedAlerts []uint64, expiry time.Duration) error {
	return l.logFunc(r, gkey, firingAlerts, resolvedAlerts, expiry)
}
