package service

import (
	"context"
	"errors"
	"github.com/kimoofey/tui/ocm/history/diagnostics"
	"sync"
	"time"

	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/session"
)

// HistoryStore keeps archive lifetime independent of source availability.
type HistoryStore interface {
	HistoryArchive
	Query(context.Context, *history.Range) (history.Report, error)
	Close() error
}
type BrowserState struct {
	App           *Service
	Snapshot      *Snapshot
	Err           error
	ImportPending bool
}
type HistoryBackend interface {
	Query(context.Context, *history.Range) (history.Report, error)
	Refresh(context.Context) BrowserState
}

// HistoryBrowser owns retries and all enabled-mode resources. Every blocking
// operation runs in a command; Close cancels and joins before closing handles.
type HistoryBrowser struct {
	archive     HistoryStore
	discover    func(context.Context) (session.Source, error)
	launcher    Launcher
	load        LoadOptions
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	closing     bool
	tasks       sync.WaitGroup
	refresh     chan struct{}
	app         *Service
	source      session.Source
	closeOnce   sync.Once
	closeErr    error
	diagnostics *diagnostics.Logger
}

func NewHistoryBrowser(archive HistoryStore, discover func(context.Context) (session.Source, error), launcher Launcher, load LoadOptions, logs ...*diagnostics.Logger) *HistoryBrowser {
	ctx, cancel := context.WithCancel(context.Background())
	var log *diagnostics.Logger
	if len(logs) > 0 {
		log = logs[0]
	}
	return &HistoryBrowser{diagnostics: log, archive: archive, discover: discover, launcher: launcher, load: load, ctx: ctx, cancel: cancel, refresh: make(chan struct{}, 1)}
}
func (b *HistoryBrowser) begin(parent context.Context) (context.Context, func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closing {
		return nil, nil, context.Canceled
	}
	b.tasks.Add(1)
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(b.ctx, cancel)
	return ctx, func() { stop(); cancel(); b.tasks.Done() }, nil
}
func (b *HistoryBrowser) Query(ctx context.Context, r *history.Range) (history.Report, error) {
	ctx, done, err := b.begin(ctx)
	if err != nil {
		return history.Report{}, err
	}
	defer done()
	start := time.Now()
	report, err := b.archive.Query(ctx, r)
	b.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Query, Code: diagnosticCode(err), Duration: time.Since(start), Count: len(report.Days)})
	return report, err
}
func (b *HistoryBrowser) Refresh(ctx context.Context) BrowserState {
	ctx, done, err := b.begin(ctx)
	if err != nil {
		return BrowserState{Err: err}
	}
	defer done()
	select {
	case b.refresh <- struct{}{}:
	case <-ctx.Done():
		return BrowserState{Err: ctx.Err()}
	}
	defer func() { <-b.refresh }()
	if b.source == nil {
		start := time.Now()
		source, err := b.discover(ctx)
		b.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Discovery, Code: diagnosticCode(err), Duration: time.Since(start)})
		if err != nil {
			if source != nil {
				source.Close()
			}
			return BrowserState{Err: err}
		}
		if source == nil {
			return BrowserState{Err: errors.New("source discovery returned no source")}
		}
		if err = ctx.Err(); err != nil {
			source.Close()
			return BrowserState{Err: err}
		}
		b.source = source
		b.app = New(source, b.launcher, nil)
		b.app.EnableHistory(b.archive, 0, b.diagnostics)
	}
	complete, err := b.InitialImportComplete(ctx)
	if err != nil {
		b.closeSource()
		return BrowserState{Err: err}
	}
	b.app.history.mu.Lock()
	b.app.history.importPending = !complete
	b.app.history.mu.Unlock()
	start := time.Now()
	snap, err := b.app.Load(ctx, b.load)
	count := 0
	if snap != nil {
		count = len(snap.Sessions)
	}
	b.diagnostics.Record(diagnostics.Event{Kind: diagnostics.SourceLoad, Code: diagnosticCode(err), Duration: time.Since(start), Count: count})
	if err != nil {
		b.closeSource()
		return BrowserState{Err: err}
	}
	return BrowserState{App: b.app, Snapshot: snap, ImportPending: !complete}
}
func (b *HistoryBrowser) closeSource() {
	if b.app != nil {
		b.app.CloseHistory()
		b.app = nil
	}
	if b.source != nil {
		b.closeErr = errors.Join(b.closeErr, b.source.Close())
		b.source = nil
	}
}
func (b *HistoryBrowser) Close() error {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closing = true
		b.cancel()
		b.mu.Unlock()
		b.tasks.Wait()
		b.closeSource()
		b.closeErr = errors.Join(b.closeErr, b.archive.Close())
	})
	return b.closeErr
}

// DiagnosticsWarning reads cached status only; it performs no file I/O and may
// be attached to the next UI query/refresh/collection response.
func (b *HistoryBrowser) DiagnosticsWarning() string { return b.diagnostics.Warning() }

// HistoryImportBackend is optional for preview and alternate history backends.
// Import must stop before returning on cancellation. Progress is available through
// the refreshed Service's HistoryStatus, which is safe to read concurrently.
type HistoryImportBackend interface {
	Import(context.Context) error
}
type initialImportStore interface {
	InitialImportComplete(context.Context) (bool, error)
	CompleteInitialImport(context.Context) error
}

func (b *HistoryBrowser) InitialImportComplete(ctx context.Context) (bool, error) {
	if store, ok := b.archive.(initialImportStore); ok {
		return store.InitialImportComplete(ctx)
	}
	return true, nil
}
func (b *HistoryBrowser) Import(ctx context.Context) error {
	ctx, done, err := b.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	select {
	case b.refresh <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-b.refresh }()
	if b.app == nil {
		return errors.New("source unavailable; refresh to retry")
	}
	h := b.app.history
	h.mu.Lock()
	if !h.importPending {
		h.mu.Unlock()
		return nil
	}
	h.status.Revision++
	h.status.Running = true
	h.status.Err = nil
	h.mu.Unlock()
	start := time.Now()
	h.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Collection, Stage: diagnostics.Begin})
	err = h.sweep(ctx, b.app.now)
	if err == nil {
		if store, ok := b.archive.(initialImportStore); ok {
			err = store.CompleteInitialImport(ctx)
		}
	}
	h.diagnostics.Record(diagnostics.Event{Kind: diagnostics.Collection, Code: diagnosticCode(err), Duration: time.Since(start)})
	h.mu.Lock()
	h.status.Revision++
	h.status.Running = false
	h.status.Err = err
	if err == nil {
		h.importPending = false
	}
	h.mu.Unlock()
	return err
}
