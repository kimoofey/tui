package ocm

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

type historyCollectedMsg struct {
	warning string
	status  service.HistoryStatus
	app     *service.Service
}

func waitHistory(app *service.Service) tea.Cmd {
	if app == nil || !app.HistoryEnabled() {
		return nil
	}
	return func() tea.Msg {
		status := app.WaitHistory()
		return historyCollectedMsg{status: status, app: app, warning: app.DiagnosticsWarning()}
	}
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
