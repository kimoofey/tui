package ocm

import (
	"context"
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
