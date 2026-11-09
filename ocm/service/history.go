package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/history/diagnostics"
	"github.com/kimoofey/tui/ocm/session"
)

// HistoryArchive is intentionally small: orchestration does not depend on SQLite.
type HistoryArchive interface {
	Capture(context.Context, session.Observation, time.Time) error
	RecordFailure(context.Context, string) error
}
type HistoryStatus struct {
	Phase                    string
	Processed, Total, Failed int
	Revision                 uint64
	Running                  bool
	LastCapture              time.Time
	Err                      error
}
type historyWorker struct {
	diagnostics   *diagnostics.Logger
	archive       HistoryArchive
	source        session.HistorySource
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	pending       chan struct{}
	events        chan struct{}
	serial        *historyGate
	tasks         sync.WaitGroup
	closing       bool
	importPending bool
	guard         session.Availability
	mu            sync.Mutex
	status        HistoryStatus
	timeout       time.Duration
}

// EnableHistory must be called during composition, before concurrent service use.
// The caller owns the archive and closes it only after CloseHistory returns.
func (s *Service) EnableHistory(archive HistoryArchive, timeout time.Duration, logs ...*diagnostics.Logger) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	source, _ := s.source.(session.HistorySource)
	var log *diagnostics.Logger
	if len(logs) > 0 {
		log = logs[0]
	}
	h := &historyWorker{diagnostics: log, archive: archive, source: source, ctx: ctx, cancel: cancel, done: make(chan struct{}), pending: make(chan struct{}, 1), timeout: timeout, events: make(chan struct{}, 1), serial: newHistoryGate()}
	s.history = h
	go h.run(s.now)
}
func (s *Service) RequestHistory() {
	if h := s.history; h != nil {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.importPending || h.closing {
			return
		}
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
			h.status.Revision++
			h.status.Running = true
			h.mu.Unlock()
			start := time.Now()
			h.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Collection, Stage: diagnostics.Begin})
			err := h.sweep(h.ctx, now)
			h.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Collection, Code: diagnosticCode(err), Duration: time.Since(start)})
			h.mu.Lock()
			h.status.Revision++
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
func (h *historyWorker) sweep(ctx context.Context, now func() time.Time) error {
	h.mu.Lock()
	h.status.Phase = "Discovering sessions"
	h.status.Processed, h.status.Total, h.status.Failed = 0, 0, 0
	h.mu.Unlock()
	if h.source == nil {
		return fmt.Errorf("%w: history extraction unsupported by source", session.ErrUnsupported)
	}
	ids, err := h.source.HistorySessions(ctx)
	h.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Collection, Stage: diagnostics.Scope, Code: diagnosticCode(err), Count: len(ids)})
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.status.Total = len(ids)
	h.status.Phase = "Capturing sessions"
	h.mu.Unlock()
	var errs []error
	for _, id := range ids {
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = h.serial.acquire(ctx, false); err != nil {
			return err
		}
		e := h.capture(ctx, id, now)
		h.serial.release()
		h.mu.Lock()
		h.status.Processed++
		if e != nil {
			h.status.Failed++
		}
		h.mu.Unlock()
		if e != nil {
			errs = append(errs, fmt.Errorf("session %q: %w", id, e))
		}

	}
	return errors.Join(append(errs, ctx.Err())...)
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
		return fmt.Errorf("%w: history extraction unsupported by source", session.ErrUnsupported)
	}
	ctx, cancel := context.WithTimeout(parent, h.timeout)
	defer cancel()
	start := time.Now()
	o, err := h.source.CaptureHistory(ctx, id)
	stage := diagnostics.Complete
	if err == nil && (!o.Present || o.SessionID != id) {
		err = errors.New("fresh complete capture unavailable: session absent or identity changed")
	}
	if err == nil {
		stage = diagnostics.Archive
		err = h.archive.Capture(ctx, o, now())
	}
	h.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Capture, Stage: stage, Code: diagnosticCode(err), Schema: diagnostics.SchemaOf(o.Binding.Schema), SessionID: id, Count: len(o.Usage), Duration: time.Since(start)})
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

func (s *Service) DiagnosticsWarning() string {
	if s.history == nil {
		return ""
	}
	return s.history.diagnostics.Warning()
}
