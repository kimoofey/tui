//go:build !mock

package ocm

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

func bulkDeleteSessions(refs []session.Ref, app *service.Service, rootOnly bool, period service.CostPeriod) tea.Cmd {
	return serviceBulkDelete(refs, app, rootOnly, period)
}
func deleteSession(ref session.Ref, app *service.Service, rootOnly bool, period service.CostPeriod) tea.Cmd {
	return serviceDelete(ref, app, rootOnly, period)
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
