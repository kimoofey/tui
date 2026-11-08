package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kimoofey/tui/ocm/session"
)

const deleteTimeout = 30 * time.Second

func (s *Service) Resume(ctx context.Context, ref session.Ref) error {
	if err := s.validate(ref); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	source, ok := s.source.(session.Resumer)
	if !ok || s.launcher == nil {
		return unavailable("resume", session.Availability{})
	}
	spec, err := source.Resume(ctx, ref.ID)
	if err != nil {
		return err
	}
	if err := spec.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.launcher.Launch(spec)
}

type DeleteResult struct {
	Count    int
	Snapshot *Snapshot
	Err      error
}

// Delete validates the whole batch before side effects and refreshes once even
// when individual deletes fail. A failed refresh returns no partial snapshot.
func (s *Service) Delete(ctx context.Context, refs []session.Ref, opts LoadOptions) DeleteResult {
	for _, ref := range refs {
		if err := s.validate(ref); err != nil {
			return DeleteResult{Err: err}
		}
	}
	if s.history != nil {
		return s.deleteWithHistory(ctx, refs, opts)
	}
	source, ok := s.source.(session.Deleter)
	if !ok {
		return DeleteResult{Err: unavailable("delete", session.Availability{})}
	}
	result := DeleteResult{}
	var errs []error
	for _, ref := range refs {
		callCtx, cancel := context.WithTimeout(ctx, deleteTimeout)
		err := callCtx.Err()
		if err == nil {
			err = source.Delete(callCtx, ref.ID)
		}
		cancel()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", ref.ID, err))
		} else {
			result.Count++
		}
	}
	snap, err := s.Load(ctx, opts)
	result.Snapshot = snap
	result.Err = errors.Join(append(errs, err)...)
	return result
}
func (s *Service) Vacuum(ctx context.Context) (session.Metrics, error) {
	if err := ctx.Err(); err != nil {
		return session.Metrics{}, err
	}
	caps := s.Capabilities()
	if !caps.Vacuum.Available {
		return session.Metrics{}, unavailable("vacuum", caps.Vacuum)
	}
	return s.source.(session.Vacuumer).Vacuum(ctx)
}
func (s *Service) Prune(ctx context.Context) (session.Orphans, error) {
	if err := ctx.Err(); err != nil {
		return session.Orphans{}, err
	}
	caps := s.Capabilities()
	if !caps.Orphans.Available {
		return session.Orphans{}, unavailable("pruning", caps.Orphans)
	}
	// A prior snapshot may be stale or have failed inspection. Inspect again before maintenance.
	source := s.source.(session.OrphanMaintainer)
	if _, err := source.InspectOrphans(ctx); err != nil {
		return session.Orphans{}, fmt.Errorf("reading orphan stats: %w", err)
	}
	return source.Prune(ctx)
}
