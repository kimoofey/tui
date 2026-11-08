package service

import (
	"context"
	"errors"
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
	serial  sync.Mutex
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
	h := &historyWorker{archive: archive, source: source, ctx: ctx, cancel: cancel, done: make(chan struct{}), pending: make(chan struct{}, 1), timeout: timeout, events: make(chan struct{}, 1)}
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
		h.cancel()
		<-h.done
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
		h.serial.Lock()
		ctx, cancel := context.WithTimeout(h.ctx, h.timeout)
		o, e := h.source.CaptureHistory(ctx, id)
		if e == nil {
			e = h.archive.Capture(ctx, o, now())
		}
		cancel()
		h.serial.Unlock()
		if e != nil {
			errs = append(errs, e)
			if !errors.Is(e, history.ErrBinding) {
				if statusErr := h.archive.RecordFailure(h.ctx, id); statusErr != nil {
					errs = append(errs, statusErr)
				}
			}
		} else {
			h.mu.Lock()
			h.status.LastCapture = now()
			h.mu.Unlock()
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
