package ocm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm/service"
)

type historyImportState struct {
	pending, offered bool
	dialog           string
	id               uint64
	cancel           context.CancelFunc
	canceling        bool
	err              error
	frame            int
}
type historyImportDoneMsg struct {
	id  uint64
	err error
}
type historyImportTickMsg struct{ id uint64 }

func importTick(id uint64) tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return historyImportTickMsg{id} })
}
func (m *Model) startHistoryImport() tea.Cmd {
	backend, ok := m.history.backend.(service.HistoryImportBackend)
	if !ok {
		return nil
	}
	s := &m.history.setup
	s.id++
	s.dialog, s.canceling, s.err = "running", false, nil
	m.history.collection = service.HistoryStatus{}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	id := s.id
	return tea.Batch(importTick(id), func() tea.Msg {
		defer cancel()
		return historyImportDoneMsg{id: id, err: backend.Import(ctx)}
	})
}
func (m *Model) leaveHistoryImport() {
	s := &m.history.setup
	s.dialog = ""
	m.history.active = !m.browserUsable()
	if s.pending {
		m.status = "History import pending; press h then r to start"
		m.statusOK = false
	}
}
func (m *Model) historyImportKey(key string) tea.Cmd {
	s := &m.history.setup
	switch key {
	case "ctrl+c":
		return tea.Quit
	case "esc":
		if s.dialog == "running" {
			if s.cancel != nil {
				s.cancel()
				s.canceling = true
			}
		} else {
			m.leaveHistoryImport()
		}
	case "enter":
		if s.dialog != "running" {
			return m.startHistoryImport()
		}
	}
	return nil
}
func (m Model) historyImportView() tea.View {
	s := m.history.setup
	lines := []string{"Import existing usage into History?", "", "Reads accounting data only. Your source database is unchanged."}
	switch s.dialog {
	case "running":
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		status := m.history.collection
		if status.Phase == "Capturing sessions" {
			width := max(1, min(36, m.width-10))
			filled := width
			if status.Total > 0 {
				filled = min(width, width*status.Processed/status.Total)
			}
			lines = append(lines, "", "["+strings.Repeat("█", filled)+strings.Repeat("░", width-filled)+"]", fmt.Sprintf("Processed %d / %d sessions · %d failed", status.Processed, status.Total, status.Failed))
		} else {
			lines = append(lines, "", frames[s.frame%len(frames)]+" Discovering sessions…")
		}
		if s.canceling {
			lines = append(lines, "Canceling scan…")
		} else {
			lines = append(lines, "", "esc Cancel scan")
		}
	case "failed":
		lines = append(lines, "", "Import incomplete: "+ui.Truncate(safeLabel(s.err.Error()), max(1, min(120, m.width-10))), "Committed captures are saved. Coverage is partial.", "", "enter Retry · esc Continue with partial history")
	default:
		lines = append(lines, "Existing captures are retained. You can cancel and retry later.", "", "enter Start scan · esc Not now")
	}
	width, height := max(1, m.width), max(1, m.height)
	content := lipgloss.NewStyle().Foreground(ui.ColorText).Border(lipgloss.RoundedBorder()).BorderForeground(ui.ColorAccent).Padding(1).Width(max(1, min(72, width-6))).Render(strings.Join(lines, "\n"))
	content = lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
	v := tea.NewView(lipgloss.NewStyle().MaxWidth(width).MaxHeight(height).Render(content))
	v.AltScreen = true
	return v
}
func (m *Model) finishHistoryImport(msg historyImportDoneMsg) tea.Cmd {
	s := &m.history.setup
	if s.dialog != "running" || s.id != msg.id {
		return nil
	}
	s.cancel = nil
	s.err = msg.err
	if m.app != nil {
		m.history.collection = m.app.HistoryStatus()
		m.capabilities = m.app.Capabilities()
		m.updateKeys()
	}
	if msg.err == nil {
		s.pending = false
		m.status = "History import complete"
		m.statusOK = true
		m.leaveHistoryImport()
	} else if s.canceling || errors.Is(msg.err, context.Canceled) {
		m.leaveHistoryImport()
	} else {
		s.dialog = "failed"
	}
	return m.history.query()
}
