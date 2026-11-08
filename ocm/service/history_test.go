package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kimoofey/tui/ocm/session"
)

type historySource struct {
	capableSource
	entered     chan string
	release     chan struct{}
	mu          sync.Mutex
	inventories int
}

func (f *historySource) HistorySessions(context.Context) ([]string, error) {
	f.mu.Lock()
	f.inventories++
	f.mu.Unlock()
	return []string{"bad", "good"}, nil
}
func (f *historySource) CaptureHistory(ctx context.Context, id string) (session.Observation, error) {
	select {
	case f.entered <- id:
	case <-ctx.Done():
		return session.Observation{}, ctx.Err()
	}
	select {
	case <-f.release:
	case <-ctx.Done():
		return session.Observation{}, ctx.Err()
	}
	if id == "bad" {
		return session.Observation{}, errors.New("fixture capture failure")
	}
	return session.Observation{Present: true, SessionID: id}, nil
}

type historyArchive struct {
	mu       sync.Mutex
	captures []string
	failed   []string
}

func (a *historyArchive) Capture(_ context.Context, o session.Observation, _ time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.captures = append(a.captures, o.SessionID)
	return nil
}
func (a *historyArchive) RecordFailure(_ context.Context, id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failed = append(a.failed, id)
	return nil
}
func TestHistoryWorkerCoalescesAndKeepsBrowserUsable(t *testing.T) {
	f := &historySource{entered: make(chan string, 8), release: make(chan struct{}, 8)}
	f.id = "fake"
	a := &historyArchive{}
	s := New(f, nil, func() time.Time { return time.Unix(1, 0) })
	s.EnableHistory(a, time.Minute)
	defer s.CloseHistory()
	s.RequestHistory()
	if id := <-f.entered; id != "bad" {
		t.Fatal(id)
	}
	// Load completes while capture is blocked; repeated requests leave one sweep pending.
	if _, err := s.Load(context.Background(), LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		s.RequestHistory()
	}
	if s.Capabilities().Delete {
		t.Fatal("unguarded delete available")
	}
	result := s.Delete(context.Background(), []session.Ref{{Source: "fake", ID: "good"}}, LoadOptions{})
	if result.Err == nil || len(f.deletes) != 0 {
		t.Fatal(result)
	}
	f.release <- struct{}{}
	if id := <-f.entered; id != "good" {
		t.Fatal(id)
	}
	f.release <- struct{}{}
	first := s.WaitHistory()
	if first.Err == nil {
		t.Fatal("failure not reported")
	}
	if id := <-f.entered; id != "bad" {
		t.Fatal(id)
	}
	f.release <- struct{}{}
	if id := <-f.entered; id != "good" {
		t.Fatal(id)
	}
	f.release <- struct{}{}
	s.WaitHistory()
	f.mu.Lock()
	count := f.inventories
	f.mu.Unlock()
	if count != 2 {
		t.Fatalf("sweeps=%d", count)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.captures) != 2 || len(a.failed) != 2 {
		t.Fatal(a.captures, a.failed)
	}
}
func TestHistoryCancellationJoins(t *testing.T) {
	f := &historySource{entered: make(chan string, 1), release: make(chan struct{})}
	s := New(f, nil, nil)
	s.EnableHistory(&historyArchive{}, time.Minute)
	s.RequestHistory()
	<-f.entered
	s.CloseHistory()
	select {
	case <-s.history.done:
	default:
		t.Fatal("worker not joined")
	}
}

func TestHistorySessionTimeoutContinues(t *testing.T) {
	f := &historySource{entered: make(chan string, 8), release: make(chan struct{})}
	a := &historyArchive{}
	s := New(f, nil, nil)
	s.EnableHistory(a, time.Millisecond)
	defer s.CloseHistory()
	s.RequestHistory()
	status := s.WaitHistory()
	if !errors.Is(status.Err, context.DeadlineExceeded) {
		t.Fatal(status.Err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.failed) != 2 || len(a.captures) != 0 {
		t.Fatal(a.failed, a.captures)
	}
}
