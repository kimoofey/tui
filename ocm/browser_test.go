package ocm

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

type browserSource struct {
	listing session.Listing
	err     error
}

func (f *browserSource) Info() session.Info { return session.Info{ID: "fake", Name: "Fake"} }
func (f *browserSource) List(context.Context, session.ListOptions) (session.Listing, error) {
	return f.listing, f.err
}
func (f *browserSource) Close() error { return nil }
func TestListOnlyBrowser(t *testing.T) {
	source := &browserSource{listing: session.Listing{Sessions: []session.Session{{Ref: session.Ref{Source: "fake", ID: "same"}, Title: "Fake session"}}, TotalCount: 1}}
	app := service.New(source, nil, nil)
	snap, err := app.Load(context.Background(), service.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	model := New(OptionsFromSnapshot(snap, app, service.LoadOptions{}), 100, 30)
	if model.keys.Enter.Enabled() || model.keys.Delete.Enabled() || model.keys.Vacuum.Enabled() || model.keys.Prune.Enabled() {
		t.Fatal("unsupported help enabled")
	}
	if got := model.table.Rows()[0][3]; got != "n/a" {
		t.Fatalf("missing cost=%q", got)
	}
	if strings.Contains(model.renderTitleBar(), " B") {
		t.Fatal("absent metrics displayed")
	}
	for _, r := range []rune{'d', 'v', 'p'} {
		updated, cmd := model.Update(tea.KeyPressMsg{Code: r})
		got := updated.(Model)
		if cmd != nil || got.deletePhase != deleteNone || got.vacuumPhase || got.prunePhase {
			t.Fatalf("unsupported key %c invoked action", r)
		}
	}
	if _, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Fatal("unsupported resume invoked")
	}
	// Full refs distinguish otherwise identical IDs in selections and acknowledgements.
	other := session.Ref{Source: "other", ID: "same"}
	model.sessions = append(model.sessions, session.Session{Ref: other})
	model.selected[source.listing.Sessions[0].Ref] = true
	model.selected[other] = true
	model.removeAcknowledgedSessions([]session.Ref{other})
	if len(model.sessions) != 1 || model.sessions[0].Source != "fake" {
		t.Fatal("acknowledgement crossed source boundary")
	}
	if formatRecordedCost(&session.RecordedCost{Amount: 0, Currency: "USD"}) != "-" {
		t.Fatal("known zero presentation changed")
	}
}
func TestIndependentBindingsAndFailedRefresh(t *testing.T) {
	enabled := service.Capabilities{Resume: true, Delete: true, Vacuum: session.Availability{Available: true}, Orphans: session.Availability{Available: true}}
	first := New(Options{Capabilities: enabled}, 100, 30)
	second := New(Options{}, 100, 30)
	if !first.keys.Enter.Enabled() || !first.keys.Delete.Enabled() || second.keys.Enter.Enabled() || !defaultKeys.Enter.Enabled() {
		t.Fatal("models share mutable key state")
	}
	source := &browserSource{listing: session.Listing{Sessions: []session.Session{{Ref: session.Ref{Source: "fake", ID: "one"}}}, TotalCount: 1}}
	app := service.New(source, nil, nil)
	snap, err := app.Load(context.Background(), service.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	model := New(OptionsFromSnapshot(snap, app, service.LoadOptions{}), 100, 30)
	model.selected[model.sessions[0].Ref] = true
	model.reloading = true
	locked, cmd := model.Update(tea.KeyPressMsg{Code: 'q'})
	if cmd != nil || !locked.(Model).reloading {
		t.Fatal("reload busy lock changed")
	}
	source.err = errors.New("cannot refresh")
	updated, _ := model.Update(reloadedMsg{err: source.err})
	failed := updated.(Model)
	if len(failed.sessions) != 1 || len(failed.selected) != 1 || failed.reloading || failed.statusOK {
		t.Fatal("failed refresh replaced displayed state")
	}
	updated, _ = failed.Update(reloadedMsg{snapshot: &service.Snapshot{}})
	empty := updated.(Model)
	if len(empty.sessions) != 0 || len(empty.selected) != 0 || empty.table.Cursor() > 0 || empty.table.Cursor() < -1 {
		t.Fatal("successful empty snapshot not authoritative")
	}
	// A nil snapshot is neither authoritative empty data nor a mock acknowledgement.
	updated, _ = model.Update(deletedMsg{sessionID: model.sessions[0].Ref})
	if len(updated.(Model).sessions) != 1 {
		t.Fatal("missing snapshot was treated as acknowledgement")
	}
}

func TestVacuumThresholdAndConfirmation(t *testing.T) {
	caps := service.Capabilities{Vacuum: session.Availability{Available: true}}
	model := New(Options{Capabilities: caps, DBUsed: 1, DBTotal: minReclaimableBytes}, 100, 30)
	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'v'})
	if cmd != nil || updated.(Model).vacuumPhase {
		t.Fatal("vacuum allowed below threshold")
	}
	model.dbTotal = minReclaimableBytes + 1
	updated, cmd = model.Update(tea.KeyPressMsg{Code: 'v'})
	if cmd != nil || !updated.(Model).vacuumPhase {
		t.Fatal("vacuum at threshold did not confirm")
	}
	canceled, cmd := updated.Update(tea.KeyPressMsg{Code: 'x'})
	if cmd != nil || canceled.(Model).vacuumPhase {
		t.Fatal("vacuum confirmation did not cancel")
	}
}
