//go:build mock

package ocm

import (
	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

func bulkDeleteSessions(ids []session.Ref, _ *service.Service, _ bool, _ service.CostPeriod) tea.Cmd {
	return func() tea.Msg {
		return bulkDeletedMsg{acknowledgement: true, count: len(ids), ids: append([]session.Ref(nil), ids...)}
	}
}

func deleteSession(sessionID session.Ref, _ *service.Service, _ bool, _ service.CostPeriod) tea.Cmd {
	return func() tea.Msg {
		return deletedMsg{acknowledgement: true, sessionID: sessionID}
	}
}

func openSession(sessionID session.Ref, _ *service.Service) tea.Cmd {
	return func() tea.Msg {
		return openedMsg{sessionID: sessionID}
	}
}

func pruneOrphans(_ *service.Service) tea.Cmd {
	return func() tea.Msg {
		return prunedMsg{count: 3, bytes: 81_920}
	}
}

func vacuumDB(_ *service.Service, beforeTotal int64) tea.Cmd {
	return func() tea.Msg {
		compacted := beforeTotal / 2
		return vacuumedMsg{dbUsed: compacted, dbTotal: compacted, beforeTotal: beforeTotal}
	}
}

func reloadData(_ *service.Service, _ bool, costPeriod service.CostPeriod) tea.Cmd {
	return func() tea.Msg {
		snap := &service.Snapshot{Metrics: &session.Metrics{Used: 2_621_440, Total: 4_194_304}, Orphans: &session.Orphans{Count: 3, Bytes: 81_920}, Capabilities: mockCapabilities()}
		if costPeriod != "" {
			snap.PeriodCost = &session.RecordedCost{Amount: 2.51, Currency: "USD", Provenance: "source-recorded"}
		}
		return reloadedMsg{snapshot: snap}
	}
}

func mockCapabilities() service.Capabilities {
	return service.Capabilities{Resume: true, Delete: true, RootFilter: true, PeriodCost: true, Metrics: true, Vacuum: session.Availability{Available: true}, Orphans: session.Availability{Available: true}}
}
