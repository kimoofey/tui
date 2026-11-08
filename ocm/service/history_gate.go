package service

import (
	"context"
	"sync"
)

// historyGate gives queued foreground deletions priority between session captures.
// Acquisition is cancellable, including during shutdown.
type historyGate struct {
	mu      sync.Mutex
	changed *sync.Cond
	held    bool
	deletes int
}

func newHistoryGate() *historyGate { g := &historyGate{}; g.changed = sync.NewCond(&g.mu); return g }
func (g *historyGate) acquire(ctx context.Context, deletion bool) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if deletion {
		g.deletes++
		g.changed.Broadcast()
		defer func() { g.deletes--; g.changed.Broadcast() }()
	}
	stop := context.AfterFunc(ctx, func() { g.mu.Lock(); g.changed.Broadcast(); g.mu.Unlock() })
	defer stop()
	for g.held || (!deletion && g.deletes > 0) {
		if err := ctx.Err(); err != nil {
			return err
		}
		g.changed.Wait()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	g.held = true
	return nil
}
func (g *historyGate) release() { g.mu.Lock(); g.held = false; g.changed.Broadcast(); g.mu.Unlock() }
