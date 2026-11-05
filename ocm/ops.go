//go:build !mock

package ocm

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

func bulkDeleteSessions(refs []session.Ref, app *service.Service, rootOnly bool, period service.CostPeriod) tea.Cmd {
	return func() tea.Msg {
		result := app.Delete(context.Background(), refs, service.LoadOptions{RootOnly: rootOnly, CostPeriod: period})
		var data *reloadedMsg
		if result.Snapshot != nil {
			data = &reloadedMsg{snapshot: result.Snapshot}
		}
		return bulkDeletedMsg{count: result.Count, data: data, err: result.Err}
	}
}
func deleteSession(ref session.Ref, app *service.Service, rootOnly bool, period service.CostPeriod) tea.Cmd {
	return func() tea.Msg {
		result := app.Delete(context.Background(), []session.Ref{ref}, service.LoadOptions{RootOnly: rootOnly, CostPeriod: period})
		var data *reloadedMsg
		if result.Snapshot != nil {
			data = &reloadedMsg{snapshot: result.Snapshot}
		}
		return deletedMsg{sessionID: ref, data: data, err: result.Err}
	}
}
func openSession(ref session.Ref, app *service.Service) tea.Cmd {
	return func() tea.Msg { return openedMsg{sessionID: ref, err: app.Resume(context.Background(), ref)} }
}
func pruneOrphans(app *service.Service) tea.Cmd {
	return func() tea.Msg {
		result, err := app.Prune(context.Background())
		return prunedMsg{count: result.Count, bytes: result.Bytes, err: err}
	}
}
func vacuumDB(app *service.Service, before int64) tea.Cmd {
	return func() tea.Msg {
		result, err := app.Vacuum(context.Background())
		return vacuumedMsg{dbUsed: result.Used, dbTotal: result.Total, beforeTotal: before, err: err}
	}
}
func reloadData(app *service.Service, rootOnly bool, period service.CostPeriod) tea.Cmd {
	return func() tea.Msg {
		snap, err := app.Load(context.Background(), service.LoadOptions{RootOnly: rootOnly, CostPeriod: period})
		return reloadedMsg{snapshot: snap, err: err}
	}
}
