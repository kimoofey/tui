package service

import (
	"context"
	"fmt"

	"github.com/kimoofey/tui/ocm/session"
)

type LoadOptions struct {
	RootOnly   bool
	CostPeriod CostPeriod
}
type Snapshot struct {
	Sessions     []session.Session
	TotalCount   int
	Capabilities Capabilities
	Metrics      *session.Metrics
	PeriodCost   *session.RecordedCost
	Orphans      *session.Orphans
	Diagnostics  []error
}

func (s *Service) Load(ctx context.Context, opts LoadOptions) (*Snapshot, error) {
	return s.load(ctx, opts, true)
}
func (s *Service) load(ctx context.Context, opts LoadOptions, collect bool) (*Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.history != nil {
		s.checkHistoryDeletion(ctx)
	}
	caps := s.Capabilities()
	if opts.RootOnly && !caps.RootFilter {
		return nil, unavailable("root filtering", session.Availability{})
	}
	var cost *session.RecordedCost
	// Resolve the clock once before queries; filtering never changes usage coverage.
	if opts.CostPeriod != "" {
		if !caps.PeriodCost {
			return nil, unavailable("period cost", session.Availability{})
		}
		start, end, err := CostPeriodBounds(opts.CostPeriod, s.now())
		if err != nil {
			return nil, fmt.Errorf("computing period bounds: %w", err)
		}
		value, err := s.source.(session.PeriodCoster).PeriodCost(ctx, start, end)
		if err != nil {
			return nil, fmt.Errorf("reading period cost (%s): %w", opts.CostPeriod, err)
		}
		cost = &value
	}
	listing, err := s.source.List(ctx, session.ListOptions{RootOnly: opts.RootOnly})
	if err != nil {
		return nil, fmt.Errorf("loading sessions: %w", err)
	}
	snap := &Snapshot{Sessions: listing.Sessions, TotalCount: listing.TotalCount, Capabilities: caps, PeriodCost: cost}
	if s.history != nil {
		s.history.mu.Lock()
		guard := s.history.guard
		s.history.mu.Unlock()
		if !guard.Available {
			snap.Diagnostics = append(snap.Diagnostics, fmt.Errorf("history deletion unavailable: %s", guard.Reason))
		}
	}
	if m, ok := s.source.(session.StorageMetrics); ok {
		value, err := m.Metrics(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading db stats: %w", err)
		}
		snap.Metrics = &value
	}
	if caps.Orphans.Available {
		value, err := s.source.(session.OrphanMaintainer).InspectOrphans(ctx)
		if err != nil {
			diagnostic := fmt.Errorf("reading orphan stats: %w", err)
			snap.Diagnostics = append(snap.Diagnostics, diagnostic)
			snap.Capabilities.Orphans = session.Availability{Reason: diagnostic.Error()}
		} else {
			snap.Orphans = &value
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if collect {
		s.RequestHistory()
	}
	return snap, nil
}
