package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/kimoofey/tui/ocm/history/diagnostics"
	"time"

	"github.com/kimoofey/tui/ocm/session"
)

type deletionGroup struct {
	target  session.Ref
	scope   []session.Ref
	members map[session.Ref]bool
}

func (s *Service) deleteWithHistory(ctx context.Context, refs []session.Ref, opts LoadOptions) (result DeleteResult) {
	start := time.Now()
	s.history.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Deletion, Stage: diagnostics.Begin, Count: len(refs)})
	defer func() {
		s.history.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Deletion, Code: diagnosticCode(result.Err), Count: result.Count, Duration: time.Since(start)})
	}()
	if _, ok := s.source.(session.HistoryDeleter); !ok || s.history.source == nil {
		return DeleteResult{Err: unavailable("history guarded deletion", session.Availability{Reason: "source does not expose a supported history deletion scope"})}
	}
	h := s.history
	h.mu.Lock()
	if h.closing {
		h.mu.Unlock()
		return DeleteResult{Err: context.Canceled}
	}
	h.tasks.Add(1)
	h.mu.Unlock()
	defer h.tasks.Done()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(h.ctx, cancel)
	defer stop()
	if err := h.serial.acquire(ctx, true); err != nil {
		return DeleteResult{Err: err}
	}
	result = s.deleteHistoryBatch(ctx, refs, opts)
	h.serial.release()
	if result.Count > 0 {
		s.RequestHistory()
	}
	return result
}

// The serializer remains held across scope discovery, capture, native completion,
// and exactly one authoritative reload, including all partial failure paths.
func (s *Service) deleteHistoryBatch(ctx context.Context, refs []session.Ref, opts LoadOptions) DeleteResult {
	var result DeleteResult
	var errs []error
	availability := s.checkHistoryDeletion(ctx)
	source, ok := s.source.(session.Deleter)
	if !ok || !availability.Available {
		errs = append(errs, unavailable("history guarded deletion", availability))
	} else {
		impact := s.source.(session.HistoryDeleter)
		var groups []deletionGroup
		seen := map[session.Ref]bool{}
		for _, ref := range refs {
			if seen[ref] {
				continue
			}
			seen[ref] = true
			callCtx, cancel := context.WithTimeout(ctx, s.history.timeout)
			start := time.Now()
			scope, err := impact.HistoryDeletionScope(callCtx, ref)
			s.history.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Deletion, Stage: diagnostics.Scope, Code: diagnosticCode(err), SessionID: ref.ID, Count: len(scope), Duration: time.Since(start)})
			cancel()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: discovering deletion scope: %w", ref.ID, err))
				continue
			}
			g := deletionGroup{target: ref, members: map[session.Ref]bool{}}
			for _, member := range scope {
				// Validate every discovered reference before any archive/native side effect.
				if err = s.validate(member); err != nil {
					return DeleteResult{Err: err}
				}
				if member.ID == "" {
					return DeleteResult{Err: errors.New("invalid empty deletion scope reference")}
				}
				if !g.members[member] {
					g.members[member] = true
					g.scope = append(g.scope, member)
				}
			}
			if !g.members[ref] {
				errs = append(errs, fmt.Errorf("%s: deletion scope omits target; action blocked", ref.ID))
				continue
			}
			groups = append(groups, g)
		}
		for i, g := range groups {
			covered := false
			for j, ancestor := range groups {
				if i != j && ancestor.members[g.target] && (!g.members[ancestor.target] || j < i) {
					covered = true
					break
				}
			}
			if covered {
				continue
			}
			var err error
			for _, ref := range g.scope {
				if err = s.history.capture(ctx, ref.ID, s.now); err != nil {
					err = fmt.Errorf("%s: history capture failed; deletion blocked: %w", ref.ID, err)
					break
				}
			}
			if err == nil {
				callCtx, cancel := context.WithTimeout(ctx, deleteTimeout)
				err = callCtx.Err()
				if err == nil {
					start := time.Now()
					err = source.Delete(callCtx, g.target.ID)
					s.history.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Deletion, Stage: diagnostics.Native, Code: diagnosticCode(err), SessionID: g.target.ID, Duration: time.Since(start)})
				}
				cancel()
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", g.target.ID, err))
			} else {
				result.Count++
			}
		}
	}
	var err error
	start := time.Now()
	result.Snapshot, err = s.load(ctx, opts, false)
	s.history.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Deletion, Stage: diagnostics.Reload, Code: diagnosticCode(err), Duration: time.Since(start)})
	result.Err = errors.Join(append(errs, err)...)
	return result
}
