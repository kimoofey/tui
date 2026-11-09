package ocm

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

type importBackendFixture struct {
	historyBackendFixture
	entered chan struct{}
	fail    bool
}

func (f *importBackendFixture) Import(ctx context.Context) error {
	if f.entered != nil {
		close(f.entered)
		<-ctx.Done()
		return ctx.Err()
	}
	if f.fail {
		return errors.New("fixture capture failure")
	}
	return nil
}
func initialImportModel(t *testing.T, f *importBackendFixture) Model {
	t.Helper()
	ref := session.Ref{Source: "fake", ID: "one"}
	f.state = service.BrowserState{App: service.New(&browserSource{}, nil, nil), Snapshot: &service.Snapshot{Sessions: []session.Session{{Ref: ref}}}, ImportPending: true}
	m := New(Options{History: f}, 100, 30)
	cmds := m.Init()().(tea.BatchMsg)
	next, _ := m.Update(cmds[2]())
	m = next.(Model)
	if m.history.setup.dialog != "prompt" {
		t.Fatal("missing prompt")
	}
	return m
}
func executeImportCommand(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	batch := cmd().(tea.BatchMsg)
	// Execute the actual blocking import and deliver its completion through Update.
	next, _ := m.Update(batch[1]())
	return next.(Model)
}
func TestInitialImportDialogDeferralAndSuccess(t *testing.T) {
	f := &importBackendFixture{}
	m := initialImportModel(t, f)
	if !strings.Contains(m.View().Content, "Import existing usage") {
		t.Fatal(m.View().Content)
	}
	for _, key := range []string{"h", "s", "r", "n", "c", "q"} {
		next, cmd := historyKeyTest(m, key)
		if next.history.setup.dialog != "prompt" || cmd != nil {
			t.Fatalf("unexpected import action for %q", key)
		}
		m = next
	}
	m, _ = historyKeyTest(m, "esc")
	if m.history.active || !m.history.setup.pending || m.history.setup.dialog != "" {
		t.Fatal(m.history.setup)
	}
	m, _ = historyKeyTest(m, "h")
	m, _ = historyKeyTest(m, "r")
	if m.history.setup.dialog != "prompt" {
		t.Fatal("manual retry missing")
	}
	m, cmd := historyKeyTest(m, "enter")
	m = executeImportCommand(t, m, cmd)
	if m.history.setup.pending || m.history.setup.dialog != "" || m.history.active {
		t.Fatal(m.history.setup)
	}
}
func TestInitialImportFailureRetryAndOldMessages(t *testing.T) {
	f := &importBackendFixture{fail: true}
	m := initialImportModel(t, f)
	m, cmd := historyKeyTest(m, "enter")
	m = executeImportCommand(t, m, cmd)
	if m.history.setup.dialog != "failed" || !strings.Contains(m.View().Content, "Continue with partial history") {
		t.Fatal(m.history.setup)
	}
	previous := m.history.setup.id
	f.fail = false
	m, cmd = historyKeyTest(m, "enter")
	next, _ := m.Update(historyImportDoneMsg{id: previous})
	m = next.(Model)
	if m.history.setup.dialog != "running" {
		t.Fatal("old completion accepted")
	}
	next, tick := m.Update(historyImportTickMsg{id: previous})
	m = next.(Model)
	if tick != nil {
		t.Fatal("stale tick renewed")
	}
	m = executeImportCommand(t, m, cmd)
	if m.history.setup.pending {
		t.Fatal("retry failed")
	}
}
func TestInitialImportCancelWaitsForCommand(t *testing.T) {
	f := &importBackendFixture{entered: make(chan struct{})}
	m := initialImportModel(t, f)
	m, cmd := historyKeyTest(m, "enter")
	batch := cmd().(tea.BatchMsg)
	done := make(chan tea.Msg, 1)
	go func() { done <- batch[1]() }()
	<-f.entered
	m, _ = historyKeyTest(m, "esc")
	if m.history.setup.dialog != "running" || !m.history.setup.canceling {
		t.Fatal("cancellation did not wait")
	}
	next, _ := m.Update(<-done)
	m = next.(Model)
	if m.history.setup.dialog != "" || !m.history.setup.pending || m.history.active {
		t.Fatal(m.history.setup)
	}
}
func TestInitialImportSourceUnavailableKeepsSavedHistory(t *testing.T) {
	f := &importBackendFixture{}
	f.state = service.BrowserState{Err: errors.New("source unavailable")}
	m := New(Options{History: f}, 100, 30)
	cmds := m.Init()().(tea.BatchMsg)
	next, _ := m.Update(cmds[2]())
	m = next.(Model)
	if m.history.setup.dialog != "" || !m.history.active {
		t.Fatal("source failure trapped setup")
	}
	// Recovery offers setup even after initial discovery failed.
	ref := session.Ref{Source: "fake", ID: "one"}
	f.state = service.BrowserState{App: service.New(&browserSource{}, nil, nil), Snapshot: &service.Snapshot{Sessions: []session.Session{{Ref: ref}}}, ImportPending: true}
	m, cmd := historyKeyTest(m, "r")
	refreshBatch := cmd().(tea.BatchMsg)
	next, _ = m.Update(refreshBatch[0]())
	m = next.(Model)
	if m.history.setup.dialog != "prompt" {
		t.Fatal("recovered source skipped setup")
	}
}

func TestInitialImportProgressAndResize(t *testing.T) {
	f := &importBackendFixture{}
	m := initialImportModel(t, f)
	m.history.setup.dialog = "running"
	m.history.collection = service.HistoryStatus{Phase: "Capturing sessions", Processed: 12, Total: 40, Failed: 2}
	if content := m.View().Content; !strings.Contains(content, "Processed 12 / 40 sessions · 2 failed") || !strings.Contains(content, "█") {
		t.Fatal(content)
	}
	for _, dialog := range []string{"prompt", "running", "failed"} {
		m.history.setup.dialog = dialog
		m.history.setup.err = errors.New(strings.Repeat("session capture failed ", 500))
		m.width, m.height = 80, 24
		if dialog == "failed" && !strings.Contains(m.View().Content, "Continue with partial history") {
			t.Fatal("long error hid retry/continue actions")
		}
		for _, size := range [][2]int{{80, 24}, {30, 12}, {1, 1}} {
			next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m = next.(Model)
			content := m.View().Content
			if lipgloss.Width(content) > size[0] || lipgloss.Height(content) > size[1] {
				t.Fatalf("dialog %s exceeded %v", dialog, size)
			}
		}
	}
}
