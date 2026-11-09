package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/kimoofey/tui/ocm/history/diagnostics"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
	"path/filepath"
)

type ownedSource struct {
	closed int
	fail   bool
	empty  bool
}

func (f *ownedSource) Info() session.Info { return session.Info{ID: "fake"} }
func (f *ownedSource) Close() error       { f.closed++; return nil }
func (f *ownedSource) List(context.Context, session.ListOptions) (session.Listing, error) {
	if f.fail {
		return session.Listing{}, errors.New("load failed")
	}
	if f.empty {
		return session.Listing{}, nil
	}
	return session.Listing{Sessions: []session.Session{{Ref: session.Ref{Source: "fake", ID: "one"}}}, TotalCount: 1}, nil
}
func TestSourceOwnership(t *testing.T) {
	for _, mode := range []string{"load failure", "empty", "normal", "program failure"} {
		t.Run(mode, func(t *testing.T) {
			source := &ownedSource{fail: mode == "load failure", empty: mode == "empty"}
			opts, err := loadSource(source, nil, service.LoadOptions{})
			if source.fail {
				if err == nil || source.closed != 1 {
					t.Fatal("load failure leaked source")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if source.closed != 0 {
				t.Fatal("loader closed live source")
			}
			called := false
			code := runBrowser(opts, func(tea.Model) error {
				called = true
				if mode == "program failure" {
					return errors.New("TUI failed")
				}
				return nil
			})
			if source.closed != 1 || called == source.empty || (code == 1) != (mode == "program failure") {
				t.Fatalf("cleanup: closed=%d program=%v code=%d", source.closed, called, code)
			}
		})
	}
}

func TestDisabledHistoryDoesNotOpenArchive(t *testing.T) {
	source := &ownedSource{}
	opts, err := loadHistorySource(source, nil, service.LoadOptions{}, false, func() (*history.Store, error) { panic("disabled archive factory called") })
	if err != nil {
		t.Fatal(err)
	}
	opts.Close()
}

func TestHistoryStartupCleanup(t *testing.T) {
	for _, mode := range []string{"archive failure", "load failure", "normal"} {
		t.Run(mode, func(t *testing.T) {
			source := &ownedSource{fail: mode == "load failure"}
			path := filepath.Join(t.TempDir(), "history.sqlite3")
			opts, err := loadHistorySource(source, nil, service.LoadOptions{}, true, func() (*history.Store, error) {
				if mode == "archive failure" {
					return nil, errors.New("fixture archive failure")
				}
				return history.Open(path)
			})
			if mode != "normal" {
				if err == nil || source.closed != 1 {
					t.Fatalf("cleanup: %v %d", err, source.closed)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if opts.Capabilities.Delete {
					t.Fatal("unguarded delete")
				}
				if err = opts.Close(); err != nil {
					t.Fatal(err)
				}
			}
			archive, err := history.Open(path)
			if err != nil {
				t.Fatal("archive lock leaked:", err)
			}
			archive.Close()
		})
	}
}

func TestEnabledStartupIndependentOfSource(t *testing.T) {
	for _, mode := range []string{"sessions", "empty", "missing", "load failure"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "history.sqlite3")
			source := &ownedSource{empty: mode == "empty", fail: mode == "load failure"}
			calls := 0
			opts, err := loadHistoryBrowser(func() (*history.Store, error) { return history.Open(path) }, func(context.Context) (session.Source, error) {
				calls++
				if mode == "missing" {
					return nil, errors.New("OpenCode missing")
				}
				return source, nil
			}, nil, service.LoadOptions{})
			if err != nil || opts.History == nil || calls != 0 {
				t.Fatal("startup touched source", opts, err, calls)
			}
			invoked := false
			code := runBrowser(opts, func(model tea.Model) error {
				invoked = true
				if _, err := opts.History.Query(context.Background(), nil); err != nil {
					t.Fatal(err)
				}
				state := opts.History.Refresh(context.Background())
				if mode == "missing" || mode == "load failure" {
					if state.Err == nil || state.App != nil {
						t.Fatal(state)
					}
				} else if state.Err != nil || state.App == nil {
					t.Fatal(state)
				}
				resized, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
				if !strings.Contains(resized.View().Content, "Recorded USD") {
					t.Fatal("no history view at archive-only startup")
				}
				return nil
			})
			if code != 0 || !invoked {
				t.Fatal("empty enabled startup exited", code)
			}
			if mode != "missing" && source.closed != 1 {
				t.Fatal("source close count", source.closed)
			}
			archive, err := history.Open(path)
			if err != nil {
				t.Fatal("archive lock leaked", err)
			}
			archive.Close()
		})
	}
}
func TestEnabledArchiveFailuresBeforeDiscovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.sqlite3")
	held, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	_, err = loadHistoryBrowser(func() (*history.Store, error) { return history.Open(path) }, func(context.Context) (session.Source, error) { panic("archive failure discovered source") }, nil, service.LoadOptions{})
	if err == nil {
		t.Fatal("locked archive accepted")
	}
	corrupt := filepath.Join(t.TempDir(), "history.sqlite3")
	if err = os.WriteFile(corrupt, []byte("invalid sqlite"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = loadHistoryBrowser(func() (*history.Store, error) { return history.Open(corrupt) }, func(context.Context) (session.Source, error) { panic("corrupt archive discovered source") }, nil, service.LoadOptions{})
	if err == nil {
		t.Fatal("corrupt archive accepted")
	}
}

func TestNewerEnabledArchiveFailsBeforeDiscovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.sqlite3")
	archive, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	archive.Close()
	writer, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = writer.Exec(`PRAGMA user_version=99`)
	writer.Close()
	if err != nil {
		t.Fatal(err)
	}
	_, err = loadHistoryBrowser(func() (*history.Store, error) { return history.Open(path) }, func(context.Context) (session.Source, error) { panic("newer archive discovered source") }, nil, service.LoadOptions{})
	if err == nil {
		t.Fatal("newer archive accepted")
	}
	_, err = loadHistoryBrowser(func() (*history.Store, error) { return nil, os.ErrPermission }, func(context.Context) (session.Source, error) { panic("unwritable archive discovered source") }, nil, service.LoadOptions{})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatal(err)
	}
}

func TestHistoryDiagnosticsOptInAndLifecycle(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			var config history.Config
			config.History.Enabled = true
			config.History.Diagnostics.Enabled = enabled
			path := filepath.Join(t.TempDir(), "history.log")
			calls := 0
			opts, err := loadHistoryWithDiagnostics(config, func() (*diagnostics.Logger, error) { calls++; return diagnostics.Open(path, "dev") }, func() (*history.Store, error) { return history.Open(filepath.Join(t.TempDir(), "history.db")) }, func(context.Context) (session.Source, error) { return nil, errors.New("private missing source") }, nil, service.LoadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if calls != map[bool]int{false: 0, true: 1}[enabled] {
				t.Fatal("wrong factory count", calls)
			}
			opts.History.Refresh(context.Background())
			opts.History.Query(context.Background(), nil)
			if err = opts.Close(); err != nil {
				t.Fatal(err)
			}
			opts.Close()
			if !enabled {
				if _, err = os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("disabled diagnostics wrote files", err)
				}
				return
			}
			b, err := os.ReadFile(path)
			if err != nil || !strings.Contains(string(b), `"event":"shutdown"`) || strings.Contains(string(b), "private missing source") {
				t.Fatal(string(b), err)
			}
			log, err := diagnostics.Open(path, "dev")
			if err != nil {
				t.Fatal("sink lock leaked", err)
			}
			log.Close()
		})
	}
}
func TestDiagnosticInitializationFailures(t *testing.T) {
	var config history.Config
	config.History.Enabled = true
	config.History.Diagnostics.Enabled = true
	_, err := loadHistoryWithDiagnostics(config, func() (*diagnostics.Logger, error) { return nil, os.ErrPermission }, func() (*history.Store, error) { panic("archive opened after diagnostic init failure") }, nil, nil, service.LoadOptions{})
	if err == nil || !strings.Contains(err.Error(), "opening history diagnostic log") {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "history.log")
	_, err = loadHistoryWithDiagnostics(config, func() (*diagnostics.Logger, error) { return diagnostics.Open(path, "dev") }, func() (*history.Store, error) { return nil, errors.New("archive failure with private path") }, nil, nil, service.LoadOptions{})
	if err == nil {
		t.Fatal("archive failure ignored")
	}
	log, err := diagnostics.Open(path, "dev")
	if err != nil {
		t.Fatal("failed startup leaked sink", err)
	}
	log.Close()
}
