package service

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/session"
)

type journeySource struct {
	mu     sync.Mutex
	rows   []session.Session
	usage  map[string]session.Observation
	closed int
	fail   bool
}

func (f *journeySource) Info() session.Info { return session.Info{ID: "fixture"} }
func (f *journeySource) Close() error       { f.mu.Lock(); defer f.mu.Unlock(); f.closed++; return nil }
func (f *journeySource) List(context.Context, session.ListOptions) (session.Listing, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return session.Listing{}, errors.New("fixture load failure")
	}
	return session.Listing{Sessions: append([]session.Session(nil), f.rows...), TotalCount: len(f.rows)}, nil
}
func (f *journeySource) HistorySessions(context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for id := range f.usage {
		ids = append(ids, id)
	}
	return ids, nil
}
func (f *journeySource) CaptureHistory(_ context.Context, id string) (session.Observation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o := f.usage[id]
	if o.SessionID == "" {
		o.SessionID = id
	}
	return o, nil
}
func (f *journeySource) HistoryDeletionAvailability(context.Context) session.Availability {
	return session.Availability{Available: true}
}
func (f *journeySource) HistoryDeletionScope(_ context.Context, r session.Ref) ([]session.Ref, error) {
	if r.ID == "root" {
		return []session.Ref{{Source: "fixture", ID: "root"}, {Source: "fixture", ID: "child"}}, nil
	}
	return []session.Ref{r}, nil
}
func (f *journeySource) Delete(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = nil
	f.usage = map[string]session.Observation{}
	return nil
}
func fixtureJourneySource() *journeySource {
	f := &journeySource{usage: map[string]session.Observation{}}
	for i, id := range []string{"root", "child"} {
		at := time.Date(2026, 1, 2+i, 0, 0, 0, 0, time.UTC).UnixMilli()
		cost := float64(i + 1)
		model := id
		provider := "fixture"
		f.rows = append(f.rows, session.Session{Ref: session.Ref{Source: "fixture", ID: id}})
		f.usage[id] = session.Observation{Binding: session.HistoryBinding{Path: "/fixture/source", Schema: "session_v2"}, Present: true, SessionID: id, Usage: []session.Usage{{Kind: "message", ID: id, Timestamp: &at, Cost: &cost, Provider: &provider, Model: &model}}}
	}
	return f
}
func TestHistoryBrowserFixtureJourney(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history.sqlite3")
	archive, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	f := fixtureJourneySource()
	b := NewHistoryBrowser(archive, func(context.Context) (session.Source, error) { return f, nil }, nil, LoadOptions{})
	state := b.Refresh(ctx)
	if state.Err != nil || len(state.Snapshot.Sessions) != 2 {
		t.Fatal(state)
	}
	if !state.ImportPending {
		t.Fatal("initial import not offered")
	}
	if err := b.Import(ctx); err != nil {
		t.Fatal(err)
	}
	r, err := b.Query(ctx, nil)
	if err != nil || r.Cost == nil || *r.Cost != 3 || len(r.Days) != 2 || len(r.Models) != 2 {
		t.Fatal(r, err)
	}
	state = b.Refresh(ctx)
	state.App.WaitHistory()
	again, err := b.Query(ctx, nil)
	if err != nil || *again.Cost != 3 || again.Rows != 2 {
		t.Fatal(again, err)
	}
	result := state.App.Delete(ctx, []session.Ref{{Source: "fixture", ID: "root"}, {Source: "fixture", ID: "child"}}, LoadOptions{})
	if result.Err != nil || result.Count != 1 || len(result.Snapshot.Sessions) != 0 {
		t.Fatal(result)
	}
	state.App.WaitHistory()
	if err = b.Close(); err != nil {
		t.Fatal(err)
	}
	if err = b.Close(); err != nil {
		t.Fatal(err)
	}
	if f.closed != 1 {
		t.Fatal("source cleanup", f.closed)
	}
	archive, err = history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	missing := NewHistoryBrowser(archive, func(context.Context) (session.Source, error) { return nil, errors.New("OpenCode missing") }, nil, LoadOptions{})
	defer missing.Close()
	if retry := missing.Refresh(ctx); retry.Err == nil || retry.App != nil {
		t.Fatal(retry)
	}
	saved, err := missing.Query(ctx, nil)
	if err != nil || *saved.Cost != 3 || len(saved.Days) != 2 {
		t.Fatal(saved, err)
	}
}
func TestHistoryBrowserQueriesDuringDiscoveryAndShutdown(t *testing.T) {
	archive, err := history.Open(filepath.Join(t.TempDir(), "history.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	done := make(chan BrowserState, 1)
	b := NewHistoryBrowser(archive, func(ctx context.Context) (session.Source, error) { close(entered); <-ctx.Done(); return nil, ctx.Err() }, nil, LoadOptions{})
	go func() { done <- b.Refresh(context.Background()) }()
	<-entered
	if _, err = b.Query(context.Background(), nil); err != nil {
		t.Fatal("saved reads blocked by source", err)
	}
	if err = b.Close(); err != nil {
		t.Fatal(err)
	}
	if state := <-done; !errors.Is(state.Err, context.Canceled) {
		t.Fatal(state)
	}
	if _, err = b.Query(context.Background(), nil); !errors.Is(err, context.Canceled) {
		t.Fatal("query after close", err)
	}
}
func TestHistoryBrowserSourceRetryAndEmptyCoverage(t *testing.T) {
	archive, err := history.Open(filepath.Join(t.TempDir(), "history.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	attempt := 0
	bad := &journeySource{fail: true}
	good := fixtureJourneySource()
	b := NewHistoryBrowser(archive, func(context.Context) (session.Source, error) {
		attempt++
		switch attempt {
		case 1:
			return nil, errors.New("executable missing")
		case 2:
			return bad, nil
		default:
			return good, nil
		}
	}, nil, LoadOptions{})
	defer b.Close()
	for i := 0; i < 2; i++ {
		state := b.Refresh(context.Background())
		if state.Err == nil || state.App != nil {
			t.Fatal(state)
		}
	}
	if bad.closed != 1 {
		t.Fatal("failed load leaked source")
	}
	r, err := b.Query(context.Background(), nil)
	if err != nil || r.Cost != nil || len(r.Sessions) != 0 {
		t.Fatal("invented coverage", r, err)
	}
	state := b.Refresh(context.Background())
	if state.Err != nil || len(state.Snapshot.Sessions) != 2 {
		t.Fatal(state)
	}
	if err := b.Import(context.Background()); err != nil {
		t.Fatal(err)
	}
	r, err = b.Query(context.Background(), nil)
	if err != nil || r.Cost == nil || *r.Cost != 3 {
		t.Fatal(r, err)
	}
}

func TestHistoryBrowserBindingMismatchKeepsSavedHistory(t *testing.T) {
	archive, err := history.Open(filepath.Join(t.TempDir(), "history.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	original := fixtureJourneySource()
	for _, o := range original.usage {
		if err = archive.Capture(ctx, o, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	other := fixtureJourneySource()
	for id, o := range other.usage {
		o.Binding.Path = "/different/source"
		other.usage[id] = o
	}
	b := NewHistoryBrowser(archive, func(context.Context) (session.Source, error) { return other, nil }, nil, LoadOptions{})
	defer b.Close()
	state := b.Refresh(ctx)
	if state.Err != nil || state.App == nil || len(state.Snapshot.Sessions) != 2 {
		t.Fatal(state)
	}
	importErr := b.Import(ctx)
	status := state.App.HistoryStatus()
	if !errors.Is(importErr, history.ErrBinding) || state.App.Capabilities().Delete {
		t.Fatal(status)
	}
	r, err := b.Query(ctx, nil)
	if err != nil || *r.Cost != 3 || r.Rows != 2 {
		t.Fatal(r, err)
	}
}
func TestHistoryBrowserUnsupportedCollectionKeepsBrowser(t *testing.T) {
	archive, err := history.Open(filepath.Join(t.TempDir(), "history.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	source := &listSource{id: "limited", listing: session.Listing{Sessions: []session.Session{{Ref: session.Ref{Source: "limited", ID: "one"}}}, TotalCount: 1}}
	b := NewHistoryBrowser(archive, func(context.Context) (session.Source, error) { return source, nil }, nil, LoadOptions{})
	defer b.Close()
	state := b.Refresh(context.Background())
	if state.Err != nil || len(state.Snapshot.Sessions) != 1 {
		t.Fatal(state)
	}
	if err := b.Import(context.Background()); err == nil || state.App.Capabilities().Delete {
		t.Fatal(err)
	}
	if _, err = b.Query(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestInitialImportDeferralEmptyAndRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history.db")
	archive, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	source := fixtureJourneySource()
	source.usage = map[string]session.Observation{}
	b := NewHistoryBrowser(archive, func(context.Context) (session.Source, error) { return source, nil }, nil, LoadOptions{})
	state := b.Refresh(ctx)
	if state.Err != nil || !state.ImportPending {
		t.Fatal(state)
	}
	// Even explicit automatic sweep requests are suppressed while setup is pending.
	state.App.RequestHistory()
	state = b.Refresh(ctx)
	if state.App.HistoryStatus().Running || !state.ImportPending {
		t.Fatal(state)
	}
	if err := b.Import(ctx); err != nil {
		t.Fatal(err)
	}
	if complete, err := archive.InitialImportComplete(ctx); err != nil || !complete {
		t.Fatal(complete, err)
	}
	if state.App.HistoryStatus().Total != 0 {
		t.Fatal(state.App.HistoryStatus())
	}
	b.Close()
	archive, err = history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	b = NewHistoryBrowser(archive, func(context.Context) (session.Source, error) { return fixtureJourneySource(), nil }, nil, LoadOptions{})
	defer b.Close()
	state = b.Refresh(ctx)
	if state.Err != nil || state.ImportPending {
		t.Fatal(state)
	}
	if status := state.App.WaitHistory(); status.Err != nil {
		t.Fatal(status)
	}
}

type cancellableImportSource struct {
	*journeySource
	entered, release chan struct{}
}

func (f *cancellableImportSource) HistorySessions(context.Context) ([]string, error) {
	return []string{"root", "child"}, nil
}
func (f *cancellableImportSource) CaptureHistory(ctx context.Context, id string) (session.Observation, error) {
	if id == "child" {
		select {
		case f.entered <- struct{}{}:
		default:
		}
		select {
		case <-ctx.Done():
			return session.Observation{}, ctx.Err()
		case <-f.release:
		}
	}
	return f.journeySource.CaptureHistory(ctx, id)
}
func TestInitialImportCancellationRetainsCapturesAndAllowsRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	archive, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	f := &cancellableImportSource{fixtureJourneySource(), make(chan struct{}, 1), make(chan struct{})}
	b := NewHistoryBrowser(archive, func(context.Context) (session.Source, error) { return f, nil }, nil, LoadOptions{})
	defer b.Close()
	state := b.Refresh(ctx)
	if state.Err != nil {
		t.Fatal(state.Err)
	}
	done := make(chan error, 1)
	go func() { done <- b.Import(ctx) }()
	<-f.entered
	status := state.App.HistoryStatus()
	if status.Processed != 1 || status.Total != 2 || !status.Running {
		t.Fatal(status)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if complete, err := archive.InitialImportComplete(context.Background()); err != nil || complete {
		t.Fatal(complete, err)
	}
	r, err := b.Query(context.Background(), nil)
	if err != nil || r.Rows != 1 || *r.Cost != 1 {
		t.Fatal(r, err)
	}
	// The worker is still alive and retry is idempotent.
	f.entered = make(chan struct{}, 1)
	close(f.release)
	if err := b.Import(context.Background()); err != nil {
		t.Fatal(err)
	}
	r, err = b.Query(context.Background(), nil)
	if err != nil || r.Rows != 2 || *r.Cost != 3 {
		t.Fatal(r, err)
	}
	state = b.Refresh(context.Background())
	if state.ImportPending {
		t.Fatal("retry did not complete setup")
	}
	state.App.WaitHistory()
}

func TestInitialImportPartialFailureDoesNotCompleteSetup(t *testing.T) {
	ctx := context.Background()
	archive, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	f := fixtureJourneySource()
	bad := f.usage["child"]
	bad.Present = false
	f.usage["child"] = bad
	b := NewHistoryBrowser(archive, func(context.Context) (session.Source, error) { return f, nil }, nil, LoadOptions{})
	defer b.Close()
	state := b.Refresh(ctx)
	if err := b.Import(ctx); err == nil {
		t.Fatal("partial failure ignored")
	}
	status := state.App.HistoryStatus()
	if status.Processed != 2 || status.Total != 2 || status.Failed != 1 || status.Running {
		t.Fatal(status)
	}
	if complete, err := archive.InitialImportComplete(ctx); err != nil || complete {
		t.Fatal(complete, err)
	}
	r, err := b.Query(ctx, nil)
	if err != nil || r.Rows != 1 {
		t.Fatal(r, err)
	}
}

func TestPendingInitialImportStillGuardsDeletion(t *testing.T) {
	ctx := context.Background()
	archive, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	f := fixtureJourneySource()
	b := NewHistoryBrowser(archive, func(context.Context) (session.Source, error) { return f, nil }, nil, LoadOptions{})
	defer b.Close()
	state := b.Refresh(ctx)
	if !state.ImportPending {
		t.Fatal(state)
	}
	result := state.App.Delete(ctx, []session.Ref{{Source: "fixture", ID: "root"}}, LoadOptions{})
	if result.Err != nil || result.Count != 1 {
		t.Fatal(result)
	}
	r, err := b.Query(ctx, nil)
	if err != nil || r.Rows != 2 || *r.Cost != 3 {
		t.Fatal(r, err)
	}
	if complete, err := archive.InitialImportComplete(ctx); err != nil || complete {
		t.Fatal("deletion marked initial import complete", complete, err)
	}
}

func TestInitialImportShutdownCancelsAndJoins(t *testing.T) {
	archive, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	f := &cancellableImportSource{fixtureJourneySource(), make(chan struct{}, 1), make(chan struct{})}
	b := NewHistoryBrowser(archive, func(context.Context) (session.Source, error) { return f, nil }, nil, LoadOptions{})
	if state := b.Refresh(context.Background()); state.Err != nil {
		t.Fatal(state.Err)
	}
	done := make(chan error, 1)
	go func() { done <- b.Import(context.Background()) }()
	<-f.entered
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
