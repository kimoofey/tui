package ocm

import (
	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/ocm/service"
)

type historyCollectedMsg struct{ status service.HistoryStatus }

func waitHistory(app *service.Service) tea.Cmd {
	return func() tea.Msg { return historyCollectedMsg{status: app.WaitHistory()} }
}
