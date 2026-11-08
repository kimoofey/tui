package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/session"
)

// HistoryArchive is intentionally small: orchestration does not depend on SQLite.
type HistoryArchive interface {
	Capture(context.Context, session.Observation, time.Time) error
	RecordFailure(context.Context, string) error
}
type HistoryStatus struct {
	Running     bool
	LastCapture time.Time
	Err         error
}
type historyWorker struct {
	archive HistoryArchive
	source  session.HistorySource
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	pending chan struct{}
	events  chan struct{}
	serial  *historyGate
	tasks   sync.WaitGroup
	closing bool
	guard   session.Availability
	mu      sync.Mutex
	status  HistoryStatus
	timeout time.Duration
}

// EnableHistory must be called during composition, before concurrent service use.
// The caller owns the archive and closes it only after CloseHistory returns.
func (s *Service) EnableHistory(archive HistoryArchive, timeout time.Duration) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	source, _ := s.source.(session.HistorySource)
	h := &historyWorker{archive: archive, source: source, ctx: ctx, cancel: cancel, done: make(chan struct{}), pending: make(chan struct{}, 1), timeout: timeout, events: make(chan struct{}, 1), serial: newHistoryGate()}
	s.history = h
	go h.run(s.now)
}
func (s *Service) RequestHistory() {
	if h := s.history; h != nil {
		select {
		case h.pending <- struct{}{}:
		default:
		}
	}
}
func (s *Service) CloseHistory() {
	if h := s.history; h != nil {
		h.mu.Lock()
		h.closing = true
		h.cancel()
		h.mu.Unlock()
		<-h.done
		h.tasks.Wait()
	}
}
func (s *Service) HistoryStatus() HistoryStatus {
	if s.history == nil {
		return HistoryStatus{}
	}
	s.history.mu.Lock()
	defer s.history.mu.Unlock()
	return s.history.status
}
func (h *historyWorker) run(now func() time.Time) {
	defer close(h.done)
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.pending:
			h.mu.Lock()
			h.status.Running = true
			h.mu.Unlock()
			err := h.sweep(now)
			h.mu.Lock()
			h.status.Running = false
			h.status.Err = err
			h.mu.Unlock()
			select {
			case h.events <- struct{}{}:
			default:
			}
		}
	}
}
func (h *historyWorker) sweep(now func() time.Time) error {
	if h.source == nil {
		return errors.New("history extraction unsupported by source")
	}
	ids, err := h.source.HistorySessions(h.ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, id := range ids {
		if err = h.ctx.Err(); err != nil {
			return err
		}
		if err = h.serial.acquire(h.ctx, false); err != nil {
			return err
		}
		e := h.capture(h.ctx, id, now)
		h.serial.release()
		if e != nil {
			errs = append(errs, fmt.Errorf("session %q: %w", id, e))
		}

	}
	return errors.Join(errs...)
}

func (s *Service) HistoryEnabled() bool { return s.history != nil }
func (s *Service) WaitHistory() HistoryStatus {
	h := s.history
	select {
	case <-h.events:
		return s.HistoryStatus()
	case <-h.done:
		return s.HistoryStatus()
	}
}

// capture is called only while the shared serializer is held.
func (h *historyWorker) capture(parent context.Context, id string, now func() time.Time) error {
	if h.source == nil {
		return errors.New("history extraction unsupported by source")
	}
	ctx, cancel := context.WithTimeout(parent, h.timeout)
	defer cancel()
	o, err := h.source.CaptureHistory(ctx, id)
	if err == nil && (!o.Present || o.SessionID != id) {
		err = errors.New("fresh complete capture unavailable: session absent or identity changed")
	}
	if err == nil {
		err = h.archive.Capture(ctx, o, now())
	}
	if err != nil {
		if !errors.Is(err, history.ErrBinding) {
			err = errors.Join(err, h.archive.RecordFailure(ctx, id))
		}
		return err
	}
	h.mu.Lock()
	h.status.LastCapture = now()
	h.mu.Unlock()
	return nil
}
func (s *Service) checkHistoryDeletion(ctx context.Context) session.Availability {
	a := session.Availability{Reason: "source does not expose a supported history deletion scope"}
	if source, ok := s.source.(session.HistoryDeleter); ok && s.history.source != nil {
		a = source.HistoryDeletionAvailability(ctx)
	}
	s.history.mu.Lock()
	s.history.guard = a
	s.history.mu.Unlock()
	return a
}
