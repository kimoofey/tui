package ocm

import (
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kimoofey/tui/internal/ui"
)

const (
	startupLoadingDelay   = 150 * time.Millisecond
	startupLoadingMinimum = 300 * time.Millisecond
)

type startupLoadingState struct {
	visible, minimumElapsed bool
	spinner                 spinner.Model
	pending                 *historySourceMsg
}

type startupLoadingShowMsg struct{}
type startupLoadingMinimumMsg struct{}

func showStartupLoading() tea.Cmd {
	return tea.Tick(startupLoadingDelay, func(time.Time) tea.Msg { return startupLoadingShowMsg{} })
}

func (m *Model) showStartupLoading() tea.Cmd {
	if !m.loadingSessions() || m.history.loading.visible {
		return nil
	}
	l := &m.history.loading
	l.visible = true
	l.spinner = spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(lipgloss.NewStyle().Foreground(ui.ColorAccent)))
	return tea.Batch(
		tea.Tick(startupLoadingMinimum, func(time.Time) tea.Msg { return startupLoadingMinimumMsg{} }),
		l.spinner.Tick,
	)
}

func (m *Model) finishStartupLoadingMinimum() tea.Cmd {
	l := &m.history.loading
	l.minimumElapsed = true
	if l.pending == nil {
		return nil
	}
	msg := *l.pending
	l.pending = nil
	return m.applyHistorySource(msg)
}
