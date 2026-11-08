package ocm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

type unusedHistoryArchive struct{}

func (unusedHistoryArchive) Capture(context.Context, session.Observation, time.Time) error {
	panic("unsupported source captured")
}
func (unusedHistoryArchive) RecordFailure(context.Context, string) error {
	panic("unsupported source wrote archive")
}
func TestHistoryCommandReportsFailureThroughUpdate(t *testing.T) {
	source := &browserSource{listing: session.Listing{Sessions: []session.Session{{Ref: session.Ref{Source: "fake", ID: "one"}}}, TotalCount: 1}}
	app := service.New(source, nil, nil)
	app.EnableHistory(unusedHistoryArchive{}, 0)
	defer app.CloseHistory()
	snap, err := app.Load(context.Background(), service.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m := New(OptionsFromSnapshot(snap, app, service.LoadOptions{}), 100, 30)
	// Initialization preserves the terminal background request alongside history work.
	if _, ok := m.Init()().(tea.BatchMsg); !ok {
		t.Fatal("initialization lost background request")
	}
	updated, cmd := m.Update(waitHistory(app)())
	m = updated.(Model)
	if cmd == nil || m.statusOK || !strings.Contains(m.status, "unsupported") {
		t.Fatal(m.status)
	}
	updated, _ = m.Update(historyCollectedMsg{status: service.HistoryStatus{}})
	if !updated.(Model).statusOK {
		t.Fatal("old history error retained after success")
	}
}

type guardedBrowserSource struct {
	browserSource
	fail  bool
	calls int
}

func (f *guardedBrowserSource) Delete(context.Context, string) error {
	f.calls++
	f.listing = session.Listing{}
	return nil
}
func (f *guardedBrowserSource) HistorySessions(context.Context) ([]string, error) { return nil, nil }
func (f *guardedBrowserSource) CaptureHistory(_ context.Context, id string) (session.Observation, error) {
	if f.fail {
		return session.Observation{}, errors.New("fixture unknown message role; capture rejected")
	}
	return session.Observation{Present: true, SessionID: id}, nil
}
func (f *guardedBrowserSource) HistoryDeletionAvailability(context.Context) session.Availability {
	return session.Availability{Available: true}
}
func (f *guardedBrowserSource) HistoryDeletionScope(_ context.Context, ref session.Ref) ([]session.Ref, error) {
	return []session.Ref{ref}, nil
}

type acceptingHistoryArchive struct{}

func (acceptingHistoryArchive) Capture(context.Context, session.Observation, time.Time) error {
	return nil
}
func (acceptingHistoryArchive) RecordFailure(context.Context, string) error { return nil }
func TestGuardedBrowserCommandsNormalAndMock(t *testing.T) {
	for _, fail := range []bool{true, false} {
		for _, bulk := range []bool{true, false} {
			ref := session.Ref{Source: "fake", ID: "one"}
			source := &guardedBrowserSource{browserSource: browserSource{listing: session.Listing{Sessions: []session.Session{{Ref: ref}}, TotalCount: 1}}, fail: fail}
			app := service.New(source, nil, nil)
			app.EnableHistory(acceptingHistoryArchive{}, 0)
			snap, err := app.Load(context.Background(), service.LoadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			m := New(OptionsFromSnapshot(snap, app, service.LoadOptions{}), 100, 30)
			m.selected[ref] = true
			m.bulkDeleting = true
			var cmd tea.Cmd
			if bulk {
				cmd = bulkDeleteSessions([]session.Ref{ref}, app, false, "")
			} else {
				cmd = deleteSession(ref, app, false, "")
			}
			updated, _ := m.Update(cmd())
			got := updated.(Model)
			if got.bulkDeleting || got.statusOK == fail {
				t.Fatal(got.status)
			}
			if fail {
				if source.calls != 0 || !got.selected[ref] || len(got.sessions) != 1 {
					t.Fatal("capture failure lost selection or deleted")
				}
			} else if source.calls != 1 || len(got.sessions) != 0 {
				t.Fatal("successful guarded action not reloaded")
			}
			app.CloseHistory()
		}
	}
}
