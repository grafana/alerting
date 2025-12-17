package stages

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
)

type PeerInfo interface {
	Position() int
}

// WaitStage waits for a certain amount of time before continuing or until the
// context is done.
type WaitStage struct {
	peer    PeerInfo
	timeout time.Duration
}

// NewWaitStage returns a new WaitStage.
func NewWaitStage(p PeerInfo, peerTimeout time.Duration) *WaitStage {
	return &WaitStage{
		peer:    p,
		timeout: peerTimeout,
	}
}

// Exec implements the Stage interface.
func (ws *WaitStage) Exec(ctx context.Context, l *slog.Logger, alerts ...*types.Alert) (context.Context, []*types.Alert, error) {
	if ws.peer == nil {
		return ctx, alerts, nil
	}
	peerPosition := ws.peer.Position()
	wait := time.Duration(peerPosition) * ws.timeout
	if wait == 0 {
		return ctx, alerts, nil
	}

	select {
	case <-time.After(wait):
	case <-ctx.Done():
		return ctx, nil, ctx.Err()
	}

	gkey, _ := notify.GroupKey(ctx)
	timeNow, _ := notify.Now(ctx)
	l.DebugContext(ctx,
		"continue pipeline after waiting",
		"aggrGroup", gkey,
		"timeout", wait,
		"peer_position", peerPosition,
		"pipeline_time", timeNow,
	)
	return ctx, alerts, nil
}
