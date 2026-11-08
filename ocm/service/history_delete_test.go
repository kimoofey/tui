package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/session"
)

type guardedFixture struct {
	inventory                           []string
	mu                                  sync.Mutex
	events                              []string
	scopes                              map[string][]string
	observations                        map[string]session.Observation
	captureFail, commitFail, deleteFail string
	reloadFail                          bool
	captureHook                         func(context.Context, string) error
	deleteHook                          func(context.Context) error
}

func (f *guardedFixture) event(e string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
}
func (f *guardedFixture) Info() session.Info { return session.Info{ID: "fake"} }
func (f *guardedFixture) Close() error       { return nil }
func (f *guardedFixture) List(context.Context, session.ListOptions) (session.Listing, error) {
	f.event("reload")
	if f.reloadFail {
		return session.Listing{}, errors.New("reload fixture failure")
	}
	return session.Listing{}, nil
}
func (f *guardedFixture) HistorySessions(context.Context) ([]string, error) { return f.inventory, nil }
func (f *guardedFixture) CaptureHistory(ctx context.Context, id string) (session.Observation, error) {
	f.event("capture:" + id)
	if f.captureHook != nil {
		if err := f.captureHook(ctx, id); err != nil {
			return session.Observation{}, err
		}
	}
	if id == f.captureFail {
		return session.Observation{}, errors.New("capture fixture failure")
	}
	if o, ok := f.observations[id]; ok {
		return o, nil
	}
	return session.Observation{SessionID: id, Present: true}, nil
}
func (f *guardedFixture) Capture(_ context.Context, o session.Observation, _ time.Time) error {
	f.event("commit:" + o.SessionID)
	if o.SessionID == f.commitFail {
		return errors.New("commit fixture failure")
	}
	return nil
}
func (f *guardedFixture) RecordFailure(context.Context, string) error { return nil }
func (f *guardedFixture) HistoryDeletionAvailability(context.Context) session.Availability {
	return session.Availability{Available: true}
}
func (f *guardedFixture) HistoryDeletionScope(_ context.Context, r session.Ref) ([]session.Ref, error) {
	f.event("scope:" + r.ID)
	ids, ok := f.scopes[r.ID]
	if !ok {
		return nil, errors.New("target absent")
	}
	var out []session.Ref
	for _, id := range ids {
		out = append(out, session.Ref{Source: "fake", ID: id})
	}
	return out, nil
}
func (f *guardedFixture) Delete(ctx context.Context, id string) error {
	f.event("delete:" + id)
	if f.deleteHook != nil {
		return f.deleteHook(ctx)
	}
	if id == f.deleteFail {
		return errors.New("native fixture failure")
	}
	return nil
}
func fixtureService(t *testing.T, f *guardedFixture) *Service {
	t.Helper()
	s := New(f, nil, nil)
	s.EnableHistory(f, 0)
	t.Cleanup(s.CloseHistory)
	return s
}
func refs(ids ...string) []session.Ref {
	var out []session.Ref
	for _, id := range ids {
		out = append(out, session.Ref{Source: "fake", ID: id})
	}
	return out
}
func TestGuardedDeleteOrderAndOverlap(t *testing.T) {
	f := &guardedFixture{scopes: map[string][]string{"root": {"root", "child", "grand"}, "child": {"child", "grand"}}}
	s := fixtureService(t, f)
	result := s.Delete(context.Background(), refs("child", "root", "root"), LoadOptions{})
	if result.Err != nil || result.Count != 1 || result.Snapshot == nil {
		t.Fatal(result)
	}
	want := []string{"scope:child", "scope:root", "capture:root", "commit:root", "capture:child", "commit:child", "capture:grand", "commit:grand", "delete:root", "reload"}
	if !reflect.DeepEqual(f.events, want) {
		t.Fatal(f.events)
	}
}
func TestGuardedDeleteFailures(t *testing.T) {
	for _, mode := range []string{"capture", "commit", "native", "absent", "reload", "cancel", "timeout", "missing observation", "stale", "binding"} {
		t.Run(mode, func(t *testing.T) {
			f := &guardedFixture{scopes: map[string][]string{"root": {"root", "child"}, "child": {"child"}, "other": {"other"}}}
			s := fixtureService(t, f)
			switch mode {
			case "capture":
				f.captureFail = "child"
			case "commit":
				f.commitFail = "child"
			case "native":
				f.deleteFail = "root"
			case "absent":
				delete(f.scopes, "root")
			case "reload":
				f.reloadFail = true
			case "missing observation":
				f.observations = map[string]session.Observation{"child": {SessionID: "child", Present: false}}
			case "stale", "binding":
				s.history.archive = failingArchive{fixture: f, binding: mode == "binding"}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancel" {
				f.captureHook = func(context.Context, string) error { cancel(); return context.Canceled }
			}
			if mode == "timeout" {
				f.deleteHook = func(ctx context.Context) error {
					deadline, ok := ctx.Deadline()
					if !ok || time.Until(deadline) > deleteTimeout {
						t.Error("native timeout missing")
					}
					return context.DeadlineExceeded
				}
			}
			result := s.Delete(ctx, refs("root", "child", "other"), LoadOptions{})
			if result.Err == nil {
				t.Fatal("failure ignored")
			}
			f.mu.Lock()
			events := append([]string(nil), f.events...)
			f.mu.Unlock()
			rootActions, childActions, reloads := 0, 0, 0
			for _, e := range events {
				if e == "delete:root" {
					rootActions++
				}
				if e == "delete:child" {
					childActions++
				}
				if e == "reload" {
					reloads++
				}
			}
			if childActions != 0 && mode != "absent" {
				t.Fatal("retried selected descendant", events)
			}
			if mode != "native" && mode != "reload" && mode != "timeout" && rootActions != 0 {
				t.Fatal("unguarded action", events)
			}
			if mode != "cancel" && reloads != 1 {
				t.Fatal("reload count", events)
			}
			if mode == "capture" || mode == "commit" || mode == "native" || mode == "absent" || mode == "missing observation" {
				if result.Count != 1 && !(mode == "absent" && result.Count == 2) {
					t.Fatal("independent group did not continue", result, events)
				}
			}
		})
	}
}

type failingArchive struct {
	fixture *guardedFixture
	binding bool
}

func (a failingArchive) Capture(context.Context, session.Observation, time.Time) error {
	if a.binding {
		return history.ErrBinding
	}
	return history.ErrStale
}
func (a failingArchive) RecordFailure(context.Context, string) error { return nil }
func TestGuardedForeignRefsHaveNoEffects(t *testing.T) {
	f := &guardedFixture{}
	s := fixtureService(t, f)
	r := append(refs("root"), session.Ref{Source: "foreign", ID: "other"})
	if result := s.Delete(context.Background(), r, LoadOptions{}); !errors.Is(result.Err, session.ErrForeignSource) {
		t.Fatal(result)
	}
	if len(f.events) != 0 {
		t.Fatal(f.events)
	}
}
func TestGuardedArchiveRetainedAfterRemovalAndReopen(t *testing.T) {
	for _, nativeFail := range []bool{false, true} {
		t.Run(fmt.Sprint(nativeFail), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "history.db")
			archive, err := history.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			cost := 2.5
			f := &guardedFixture{scopes: map[string][]string{"root": {"root"}}, observations: map[string]session.Observation{"root": {SessionID: "root", Present: true, Binding: session.HistoryBinding{Path: "/fixture/source", Schema: "session_v2"}, Usage: []session.Usage{{Kind: "message", ID: "usage", Cost: &cost}}}}}
			s := New(f, nil, nil)
			s.EnableHistory(archive, 0)
			f.deleteHook = func(context.Context) error {
				delete(f.observations, "root")
				if nativeFail {
					return errors.New("native failure after partial removal")
				}
				return nil
			}
			if result := s.Delete(context.Background(), refs("root"), LoadOptions{}); (result.Err != nil) != nativeFail {
				t.Fatal(result)
			}
			s.CloseHistory()
			archive.Close()
			archive, err = history.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			r, err := archive.Query(context.Background(), nil)
			if err != nil || r.Cost == nil || *r.Cost != cost {
				t.Fatal(r, err)
			}
		})
	}
}

func TestGuardedDeleteYieldsBetweenBackgroundSessions(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f := &guardedFixture{inventory: []string{"root", "other"}, scopes: map[string][]string{"root": {"root"}}}
	f.captureHook = func(ctx context.Context, id string) error {
		if id == "root" {
			once.Do(func() {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
				}
			})
		}
		return ctx.Err()
	}
	s := fixtureService(t, f)
	s.RequestHistory()
	<-entered
	done := make(chan DeleteResult, 1)
	go func() { done <- s.Delete(context.Background(), refs("root"), LoadOptions{}) }()
	// Wait until foreground deletion is queued, without a timing assumption.
	g := s.history.serial
	g.mu.Lock()
	for g.deletes == 0 {
		g.changed.Wait()
	}
	g.mu.Unlock()
	close(release)
	result := <-done
	if result.Err != nil {
		t.Fatal(result)
	}
	s.CloseHistory()
	f.mu.Lock()
	defer f.mu.Unlock()
	deletion, reload, next := -1, -1, -1
	for i, e := range f.events {
		if e == "delete:root" {
			deletion = i
		}
		if e == "reload" {
			reload = i
		}
		if e == "capture:other" && next < 0 {
			next = i
		}
	}
	if deletion < 0 || reload < deletion || (next >= 0 && next < reload) {
		t.Fatal("background capture overtook deletion/reload", f.events)
	}
}
func TestHistoryShutdownCancelsAndJoinsNativeAction(t *testing.T) {
	entered := make(chan struct{})
	f := &guardedFixture{scopes: map[string][]string{"root": {"root"}}}
	f.deleteHook = func(ctx context.Context) error { close(entered); <-ctx.Done(); return ctx.Err() }
	s := fixtureService(t, f)
	done := make(chan DeleteResult, 1)
	go func() { done <- s.Delete(context.Background(), refs("root"), LoadOptions{}) }()
	<-entered
	s.CloseHistory()
	result := <-done
	if !errors.Is(result.Err, context.Canceled) {
		t.Fatal(result)
	}
}
