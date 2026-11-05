//go:build !mock

package ocm

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

type deletingBrowserSource struct {
	browserSource
	fail, refreshFail bool
	calls             int
}

func (f *deletingBrowserSource) Delete(context.Context, string) error {
	f.calls++
	f.listing = session.Listing{}
	if f.refreshFail {
		f.err = errors.New("reload failed")
	}
	if f.fail {
		return errors.New("delete failed")
	}
	return nil
}
func TestBrowserRunsCommandsAndAppliesDeleteRefresh(t *testing.T) {
	for _, mode := range []string{"success", "partial failure", "refresh failure"} {
		t.Run(mode, func(t *testing.T) {
			ref := session.Ref{Source: "fake", ID: "one"}
			source := &deletingBrowserSource{browserSource: browserSource{listing: session.Listing{Sessions: []session.Session{{Ref: ref}}, TotalCount: 1}}, fail: mode == "partial failure", refreshFail: mode == "refresh failure"}
			app := service.New(source, nil, nil)
			snap, err := app.Load(context.Background(), service.LoadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			model := New(OptionsFromSnapshot(snap, app, service.LoadOptions{}), 100, 30)
			first, cmd := model.Update(tea.KeyPressMsg{Code: 'd'})
			if cmd != nil || first.(Model).deletePhase != deletePending {
				t.Fatal("first delete did not confirm")
			}
			next, cmd := first.Update(tea.KeyPressMsg{Code: 'd'})
			model = next.(Model)
			if cmd == nil || !model.bulkDeleting {
				t.Fatal("confirmed delete did not start")
			}
			updated, _ := model.Update(cmd())
			got := updated.(Model)
			if source.calls != 1 || got.bulkDeleting || got.statusOK != (mode == "success") {
				t.Fatalf("delete result: %+v", got)
			}
			if mode == "refresh failure" {
				if len(got.sessions) != 1 {
					t.Fatal("failed refresh lost row")
				}
			} else if len(got.sessions) != 0 || got.totalSessions != 0 {
				t.Fatal("authoritative refresh not applied")
			}
		})
	}
}
func TestBrowserBulkDeleteClearsSelectionsAfterPartialFailure(t *testing.T) {
	ref := session.Ref{Source: "fake", ID: "one"}
	source := &deletingBrowserSource{browserSource: browserSource{listing: session.Listing{Sessions: []session.Session{{Ref: ref}}, TotalCount: 1}}, fail: true}
	app := service.New(source, nil, nil)
	snap, err := app.Load(context.Background(), service.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	model := New(OptionsFromSnapshot(snap, app, service.LoadOptions{}), 100, 30)
	model.selected[ref] = true
	first, _ := model.Update(tea.KeyPressMsg{Code: 'd'})
	second, cmd := first.Update(tea.KeyPressMsg{Code: 'd'})
	updated, _ := second.Update(cmd())
	got := updated.(Model)
	if len(got.selected) != 0 || len(got.sessions) != 0 || got.statusOK {
		t.Fatal("partial bulk failure left stale state")
	}
}
func TestBrowserExecutesReload(t *testing.T) {
	source := &browserSource{listing: session.Listing{Sessions: []session.Session{{Ref: session.Ref{Source: "fake", ID: "one"}}}, TotalCount: 1}}
	app := service.New(source, nil, nil)
	snap, err := app.Load(context.Background(), service.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	model := New(OptionsFromSnapshot(snap, app, service.LoadOptions{}), 100, 30)
	source.listing = session.Listing{}
	waiting, cmd := model.Update(tea.KeyPressMsg{Code: 'r'})
	if !waiting.(Model).reloading || cmd == nil {
		t.Fatal("reload not started")
	}
	updated, _ := waiting.Update(cmd())
	if len(updated.(Model).sessions) != 0 || updated.(Model).reloading {
		t.Fatal("reload not applied")
	}
}
