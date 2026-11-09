package ocm

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

func TestStartupLoadingMinimumAndFallbacks(t *testing.T) {
	for _, outcome := range []string{"sessions", "empty", "error", "import"} {
		for _, readyAfterMinimum := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/readyAfterMinimum=%t", outcome, readyAfterMinimum), func(t *testing.T) {
				state := service.BrowserState{App: service.New(&browserSource{}, nil, nil), Snapshot: &service.Snapshot{}}
				switch outcome {
				case "sessions", "import":
					state.Snapshot.Sessions = []session.Session{{Ref: session.Ref{Source: "fake", ID: "one"}}}
					state.ImportPending = outcome == "import"
				case "error":
					state = service.BrowserState{Err: errors.New("source unavailable")}
				}
				m := historyTestModel(&historyBackendFixture{state: state})
				cmds := m.Init()().(tea.BatchMsg)
				next, _ := m.Update(startupLoadingShowMsg{})
				m = next.(Model)
				assertLoader := func() {
					t.Helper()
					content := m.View().Content
					if !strings.Contains(content, "Loading sessions…") || strings.Contains(content, "history") || strings.Contains(content, "quit") {
						t.Fatalf("unexpected loader: %q", content)
					}
				}
				assertLoader()
				if readyAfterMinimum {
					next, _ = m.Update(startupLoadingMinimumMsg{})
					m = next.(Model)
					assertLoader()
				}
				next, _ = m.Update(cmds[2]())
				m = next.(Model)
				if !readyAfterMinimum {
					assertLoader()
					if m.app != nil || m.history.setup.dialog != "" {
						t.Fatal("source result applied before minimum elapsed")
					}
					next, _ = m.Update(startupLoadingMinimumMsg{})
					m = next.(Model)
				}
				if m.loadingSessions() || m.history.loading.pending != nil {
					t.Fatal("finished load retained startup state")
				}
				content := m.View().Content
				switch outcome {
				case "sessions":
					if m.app == nil || m.history.active || strings.Contains(content, "Loading sessions") {
						t.Fatal("browser did not open")
					}
				case "empty", "error":
					if !strings.Contains(content, "Recorded USD") || outcome == "error" && !strings.Contains(content, "source unavailable") {
						t.Fatalf("missing history fallback: %q", content)
					}
				case "import":
					if !strings.Contains(content, "Import existing usage") {
						t.Fatal("missing import prompt")
					}
				}
				if _, cmd := m.Update(m.history.loading.spinner.Tick()); cmd != nil {
					t.Fatal("spinner kept ticking after startup")
				}
			})
		}
	}
}

func TestStartupLoadingCommandsAndAnimation(t *testing.T) {
	m := historyTestModel(&historyBackendFixture{})
	init := m.Init()().(tea.BatchMsg)
	next, cmd := m.Update(init[3]()) // Run the actual delayed-appearance command.
	m = next.(Model)
	if !m.history.loading.visible || cmd == nil {
		t.Fatal("delayed command did not show loader")
	}
	shown := m.View().Content
	started := cmd().(tea.BatchMsg)
	next, tick := m.Update(started[1]())
	m = next.(Model)
	if tick == nil || m.View().Content == shown {
		t.Fatal("spinner did not animate")
	}
	next, _ = m.Update(init[2]())
	m = next.(Model)
	if !m.loadingSessions() {
		t.Fatal("result bypassed minimum visibility")
	}
	next, _ = m.Update(started[0]()) // Run the actual minimum-display timer.
	m = next.(Model)
	if m.loadingSessions() {
		t.Fatal("minimum-display timer did not release result")
	}
}

func TestStartupLoadingCenteredOnResize(t *testing.T) {
	m := historyTestModel(&historyBackendFixture{})
	next, _ := m.Update(startupLoadingShowMsg{})
	m = next.(Model)
	for _, size := range [][2]int{{100, 30}, {81, 25}, {40, 10}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(Model)
		content := ansi.Strip(m.View().Content)
		lines := strings.Split(content, "\n")
		if lipgloss.Width(content) != size[0] || len(lines) != size[1] {
			t.Fatalf("loader does not fill %v", size)
		}
		found := false
		for row, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if found || !strings.Contains(line, "Loading sessions…") {
				t.Fatal("expected one loading line")
			}
			found = true
			left := len(line) - len(strings.TrimLeft(line, " "))
			right := len(line) - len(strings.TrimRight(line, " "))
			if max(left, right)-min(left, right) > 1 || row < (len(lines)-1)/2 || row > len(lines)/2 {
				t.Fatalf("loader is not centered at %v: row=%d left=%d right=%d", size, row, left, right)
			}
		}
		if !found {
			t.Fatal("missing loading line")
		}
	}
}

func TestStartupLoadingFastLoadIgnoresLateTimer(t *testing.T) {
	m := historyTestModel(&historyBackendFixture{})
	cmds := m.Init()().(tea.BatchMsg)
	next, _ := m.Update(cmds[2]())
	m = next.(Model)
	before := m.View().Content
	next, cmd := m.Update(cmds[3]())
	m = next.(Model)
	if cmd != nil || m.history.loading.visible || m.View().Content != before {
		t.Fatal("late timer showed loader after fast completion")
	}
}
