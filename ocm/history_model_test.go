package ocm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

type historyBackendFixture struct {
	report  history.Report
	err     error
	state   service.BrowserState
	queries int
}

func (f *historyBackendFixture) Query(context.Context, *history.Range) (history.Report, error) {
	f.queries++
	return f.report, f.err
}
func (f *historyBackendFixture) Refresh(context.Context) service.BrowserState { return f.state }
func historyTestModel(f *historyBackendFixture) Model {
	return New(Options{History: f, Now: func() time.Time { return time.Date(2026, 1, 1, 0, 30, 0, 0, time.FixedZone("east", 3600)) }}, 100, 30)
}
func historyKeyTest(m Model, key string) (Model, tea.Cmd) {
	var msg tea.KeyPressMsg
	if len(key) == 1 {
		msg = tea.KeyPressMsg{Code: rune(key[0])}
	} else {
		switch key {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "ctrl+c":
			msg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
		}
	}
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func TestHistoryStartupScreenTransitions(t *testing.T) {
	for _, queryFirst := range []bool{false, true} {
		for _, outcome := range []string{"sessions", "empty", "unavailable", "import"} {
			for _, openHistory := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/queryFirst=%t/history=%t", outcome, queryFirst, openHistory), func(t *testing.T) {
					app := service.New(&browserSource{}, nil, nil)
					state := service.BrowserState{App: app, Snapshot: &service.Snapshot{}}
					if outcome == "unavailable" {
						state = service.BrowserState{Err: errors.New("source unavailable")}
					} else if outcome != "empty" {
						state.Snapshot.Sessions = []session.Session{{Ref: session.Ref{Source: "fake", ID: "one"}}}
						state.ImportPending = outcome == "import"
					}
					f := &historyBackendFixture{state: state}
					m := historyTestModel(f)
					assertLoading := func() {
						t.Helper()
						view := m.View()
						if !view.AltScreen || view.Content != "" {
							t.Fatalf("unexpected startup view: %q", view.Content)
						}
					}
					assertLoading()
					cmds := m.Init()().(tea.BatchMsg)
					if openHistory {
						var cmd tea.Cmd
						m, cmd = historyKeyTest(m, "h")
						next, _ := m.Update(cmd())
						m = next.(Model)
						if !strings.Contains(m.View().Content, "Recorded USD") {
							t.Fatal("explicit history navigation did not show saved usage")
						}
					}
					order := []int{2, 1}
					if queryFirst {
						order = []int{1, 2}
					}
					for _, index := range order {
						next, _ := m.Update(cmds[index]())
						m = next.(Model)
						if index == 1 && queryFirst && !openHistory {
							assertLoading()
						}
					}
					if m.loadingSessions() {
						t.Fatal("source result did not finish loading")
					}
					wantHistory := openHistory || outcome == "empty" || outcome == "unavailable"
					if m.history.active != wantHistory {
						t.Fatalf("history active=%t, want %t", m.history.active, wantHistory)
					}
					content := m.View().Content
					switch {
					case outcome == "import":
						if !strings.Contains(content, "Import existing usage") {
							t.Fatal("startup skipped import prompt")
						}
					case wantHistory:
						if !strings.Contains(content, "Recorded USD") {
							t.Fatal("fallback/explicit history not rendered")
						}
					case strings.Contains(content, "Recorded USD"), strings.Contains(content, "Loading sessions"):
						t.Fatalf("sessions did not open: %q", content)
					}
					if outcome == "unavailable" && !strings.Contains(content, "source unavailable") {
						t.Fatal("fallback hid source failure")
					}
				})
			}
		}
	}
}

func TestHistoryStartupLoadingInputAndResize(t *testing.T) {
	m := historyTestModel(&historyBackendFixture{})
	for _, key := range []string{"enter", " ", "d", "v", "p", "r", "]", "a", "?", "esc", "down"} {
		next, cmd := historyKeyTest(m, key)
		if cmd != nil || !next.loadingSessions() || next.history.interacted || next.history.pendingRefresh || next.foregroundBusy() || next.help.ShowAll {
			t.Fatalf("loading accepted action %q", key)
		}
		m = next
	}
	for _, key := range []string{"q", "ctrl+c"} {
		_, cmd := historyKeyTest(m, key)
		if cmd == nil {
			t.Fatalf("%q did not quit", key)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%q returned a non-quit command", key)
		}
	}
	next, _ := m.Update(startupLoadingShowMsg{})
	m = next.(Model)
	for _, size := range [][2]int{{100, 30}, {20, 2}, {1, 1}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(Model)
		view := m.View()
		if !view.AltScreen || lipgloss.Width(view.Content) > size[0] || lipgloss.Height(view.Content) > size[1] {
			t.Fatalf("loading exceeded %v: %q", size, view.Content)
		}
	}
}

func TestHistoryInitAndRangesIgnoreOldQueries(t *testing.T) {
	cost := 1.25
	f := &historyBackendFixture{report: history.Report{Total: history.Total{Cost: &cost, Rows: 1}}}
	m := historyTestModel(f)
	if m.history.month.Month() != time.December || m.history.month.Year() != 2025 {
		t.Fatal(m.history.month)
	}
	cmds := m.Init()().(tea.BatchMsg)
	if len(cmds) != 4 {
		t.Fatal("background/query/source/delayed-loader commands missing")
	}
	next, _ := m.Update(cmds[1]())
	m = next.(Model)
	if !m.history.hasReport {
		t.Fatal("initial response ignored")
	}
	m, _ = historyKeyTest(m, "h")
	m, older := historyKeyTest(m, "]")
	if m.history.month.Month() != time.January {
		t.Fatal(m.history.month)
	}
	m, newer := historyKeyTest(m, "]")
	newMsg := newer()
	oldMsg := older()
	next, _ = m.Update(newMsg)
	m = next.(Model)
	next, _ = m.Update(oldMsg)
	m = next.(Model)
	if m.history.shownRange != "February 2026" {
		t.Fatal(m.history.shownRange)
	}
	m, cmd := historyKeyTest(m, "a")
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !m.history.all {
		t.Fatal("all time not selected")
	}
	m, cmd = historyKeyTest(m, "[")
	if m.history.all || m.history.month.Month() != time.December {
		t.Fatal("all-time month return", m.history)
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	f.err = errors.New("fixture query error")
	m, cmd = historyKeyTest(m, "0")
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !m.history.hasReport || m.history.queryErr == nil || *m.history.report.Cost != cost {
		t.Fatal("failed query discarded prior data")
	}
}
func TestHistorySourceRecoveryAndSelection(t *testing.T) {
	source := &browserSource{}
	app := service.New(source, nil, nil)
	ref := session.Ref{Source: "fake", ID: "one"}
	snap := &service.Snapshot{Sessions: []session.Session{{Ref: ref}, {Ref: session.Ref{Source: "fake", ID: "two"}}}, TotalCount: 2}
	f := &historyBackendFixture{state: service.BrowserState{App: app, Snapshot: snap}}
	m := historyTestModel(f)
	next, _ := m.Update(m.history.refreshCommand(1)())
	m = next.(Model)
	if m.history.active || m.app != app {
		t.Fatal("source recovery did not show browser")
	}
	m.selected[ref] = true
	m.table.SetCursor(1)
	m, _ = historyKeyTest(m, "h")
	if !m.history.active {
		t.Fatal("h did not show history")
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 70, Height: 15})
	m = next.(Model)
	m, _ = historyKeyTest(m, "h")
	if m.history.active || !m.selected[ref] || m.table.Cursor() != 1 {
		t.Fatal("switch/resize lost browser selection")
	}
	for _, mode := range []string{"delete", "vacuum", "prune", "busy"} {
		copy := m
		switch mode {
		case "delete":
			copy.deletePhase = deletePending
		case "vacuum":
			copy.vacuumPhase = true
		case "prune":
			copy.prunePhase = true
		case "busy":
			copy.bulkDeleting = true
		}
		copy, cmd := historyKeyTest(copy, "h")
		if copy.history.active || cmd != nil {
			t.Fatal(mode, "confirmation allowed switch")
		}
	}
	m.history.refreshing = false
	f.state = service.BrowserState{Err: errors.New("missing source")}
	cmd := m.history.refresh()
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !m.history.active || m.app != nil || m.history.sourceErr == nil {
		t.Fatal("failed retry did not leave archive view")
	}
	m, _ = historyKeyTest(m, "esc")
	if !m.history.active {
		t.Fatal("escaped with unavailable source")
	}
}
func TestHistoryRefreshCoalescesAndInactiveResults(t *testing.T) {
	f := &historyBackendFixture{}
	m := historyTestModel(f)
	m, _ = historyKeyTest(m, "h")
	m, first := historyKeyTest(m, "r")
	if !m.history.pendingRefresh || first == nil {
		t.Fatal("pending refresh missing")
	}
	for i := 0; i < 20; i++ {
		m, _ = historyKeyTest(m, "r")
	}
	next, cmd := m.Update(historySourceMsg{id: 1, state: service.BrowserState{Err: errors.New("offline")}})
	m = next.(Model)
	if !m.history.refreshing || m.history.pendingRefresh || m.history.refreshID != 2 || cmd == nil {
		t.Fatal("did not run one coalesced retry")
	}
	m.history.active = false
	next, _ = m.Update(historyQueriedMsg{id: m.history.queryID, label: "saved", report: history.Report{}})
	m = next.(Model)
	if !m.history.hasReport || m.history.shownRange != "saved" {
		t.Fatal("inactive query lost")
	}
}
func TestHistoryRenderingUnknownZeroQualityAndGeometry(t *testing.T) {
	zero := 0.0
	name := strings.Repeat("模型", 30)
	f := &historyBackendFixture{}
	m := historyTestModel(f)
	m, _ = historyKeyTest(m, "h")
	m.history.refreshing = false
	m.history.hasReport = true
	m.history.querying = false
	m.history.shownRange = "December 2025"
	m.history.report = history.Report{Total: history.Total{Cost: &zero, Rows: 2, UnknownCost: 1}, Days: []history.Day{{UTC: "2025-12-31", Total: history.Total{Cost: &zero, Rows: 1}}}, Models: []history.Model{{Name: &name, Total: history.Total{Rows: 1, UnknownCost: 1}}}, Sessions: []history.SessionStatus{{ID: "s", Captured: 1}}, Unbucketed: 1, ApproximateV1: true}
	text := strings.Join(m.history.lines(100), "\n")
	for _, want := range []string{"$0.00", "unknown", "Unbucketed timestamps: 1", "Approximate: V1", "Last capture:", "UTC"} {
		if !strings.Contains(text, want) {
			t.Fatal(want, text)
		}
	}
	for _, w := range []int{1, 8, 20, 80} {
		for _, h := range []int{1, 2, 5, 30} {
			copy := m
			copy.width = w
			copy.height = h
			view := copy.View()
			if lipgloss.Width(view.Content) > w || lipgloss.Height(view.Content) > h {
				t.Fatalf("geometry %dx%d: %q", w, h, view.Content)
			}
		}
	}
	m.history.report = history.Report{}
	if !strings.Contains(strings.Join(m.history.lines(80), "\n"), "No captured usage") {
		t.Fatal("empty history fabricated zero")
	}
}

func TestHistoryCollectionIgnoresOldSourceAndRevision(t *testing.T) {
	f := &historyBackendFixture{}
	m := historyTestModel(f)
	app := service.New(&browserSource{}, nil, nil)
	old := service.New(&browserSource{}, nil, nil)
	m.app = app
	m.history.collection = service.HistoryStatus{Revision: 5}
	next, cmd := m.Update(historyCollectedMsg{app: old, status: service.HistoryStatus{Revision: 20, Err: errors.New("old source")}})
	m = next.(Model)
	if cmd != nil || m.history.collection.Err != nil {
		t.Fatal("old source overwrote status")
	}
	// A stale display response cannot replace a newer completed collection.
	next, _ = m.Update(historyCollectedMsg{app: app, status: service.HistoryStatus{Revision: 4, Err: errors.New("old failure")}})
	m = next.(Model)
	if m.history.collection.Revision != 5 || m.history.collection.Err != nil {
		t.Fatal("older response replaced success")
	}
}

func TestHistoryHelpVisibilityAndDiagnosticWarning(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		f := &historyBackendFixture{}
		opts := Options{}
		if enabled {
			opts.History = f
		}
		m := New(opts, 120, 30)
		short := m.help.ShortHelpView(m.keys.ShortHelp())
		full := m.help.FullHelpView(m.keys.FullHelp())
		for _, rendered := range []string{short, full} {
			if strings.Contains(rendered, "history") != enabled {
				t.Fatalf("enabled=%v help=%q", enabled, rendered)
			}
		}
	}
	f := &historyBackendFixture{}
	m := historyTestModel(f)
	next, _ := m.Update(historyQueriedMsg{id: m.history.queryID, report: history.Report{}, warning: "History diagnostic logging stopped after a write failure"})
	m = next.(Model)
	if !strings.Contains(strings.Join(m.history.lines(100), "\n"), "diagnostic logging stopped") {
		t.Fatal("warning not rendered")
	}
}

type warningBackendFixture struct{ historyBackendFixture }

func (f *warningBackendFixture) DiagnosticsWarning() string {
	return "History diagnostic logging stopped after a write failure"
}
func TestDiagnosticWarningTravelsThroughCommands(t *testing.T) {
	f := &warningBackendFixture{}
	m := New(Options{History: f}, 100, 30)
	cmds := m.Init()().(tea.BatchMsg)
	next, _ := m.Update(cmds[1]())
	m = next.(Model)
	if !strings.Contains(m.history.diagnosticsWarning, "logging stopped") {
		t.Fatal("query dropped warning")
	}
	m.history.diagnosticsWarning = ""
	next, _ = m.Update(cmds[2]())
	m = next.(Model)
	if !strings.Contains(m.history.diagnosticsWarning, "logging stopped") {
		t.Fatal("refresh dropped warning")
	}
}
