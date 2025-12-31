package stages

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/types"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/alerting/utils"
)

type mockPeer struct {
	position int
}

func (m *mockPeer) Position() int {
	return m.position
}

func TestWaitStageExec(t *testing.T) {
	alerts := []*types.Alert{{}, {}, {}}
	tests := []struct {
		name           string
		peer           PeerInfo
		timeout        time.Duration
		contextTimeout time.Duration
		expectedErr    error
	}{
		{
			name:        "should not wait if no peer",
			peer:        nil,
			timeout:     10 * time.Second,
			expectedErr: nil,
		},
		{
			name:        "should not wait if with zero position",
			peer:        &mockPeer{position: 0},
			timeout:     10 * time.Second,
			expectedErr: nil,
		},
		{
			name:        "should wait for peer*timeout if peer with non-zero position",
			peer:        &mockPeer{position: 1},
			timeout:     100 * time.Millisecond,
			expectedErr: nil,
		},
		{
			name:           "Context timeout",
			peer:           &mockPeer{position: 2},
			timeout:        time.Second,
			contextTimeout: 100 * time.Millisecond,
			expectedErr:    context.DeadlineExceeded,
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

			logger := log.NewNopLogger()
			ws := &WaitStage{
				peer:    tc.peer,
				timeout: tc.timeout,
			}

			gotCtx, gotAlerts, gotErr := ws.Exec(ctx, utils.SlogFromGoKit(logger), alerts...)

			assert.Equal(t, ctx, gotCtx)
			if tc.expectedErr != nil {
				assert.ErrorIs(t, gotErr, tc.expectedErr)
			} else {
				assert.NoError(t, gotErr)
				assert.Equal(t, alerts, gotAlerts)
			}
		})
	}
}
