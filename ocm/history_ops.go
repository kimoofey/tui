package ocm

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

type historyCollectedMsg struct{ status service.HistoryStatus }

func waitHistory(app *service.Service) tea.Cmd {
	return func() tea.Msg { return historyCollectedMsg{status: app.WaitHistory()} }
}

func serviceBulkDelete(refs []session.Ref, app *service.Service, rootOnly bool, period service.CostPeriod) tea.Cmd {
	return func() tea.Msg {
		result := app.Delete(context.Background(), refs, service.LoadOptions{RootOnly: rootOnly, CostPeriod: period})
		var data *reloadedMsg
		if result.Snapshot != nil {
			data = &reloadedMsg{snapshot: result.Snapshot}
		}
		return bulkDeletedMsg{count: result.Count, data: data, err: result.Err}
	}
}
func serviceDelete(ref session.Ref, app *service.Service, rootOnly bool, period service.CostPeriod) tea.Cmd {
	return func() tea.Msg {
		result := app.Delete(context.Background(), []session.Ref{ref}, service.LoadOptions{RootOnly: rootOnly, CostPeriod: period})
		var data *reloadedMsg
		if result.Snapshot != nil {
			data = &reloadedMsg{snapshot: result.Snapshot}
		}
		return deletedMsg{sessionID: ref, data: data, err: result.Err}
	}
}
