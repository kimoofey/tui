//go:build mock

package ocm

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kimoofey/tui/ocm/db"
)

func bulkDeleteSessions(ids []string, _ *db.Store, _ bool) tea.Cmd {
	return func() tea.Msg {
		return bulkDeletedMsg{count: len(ids), sessions: []db.Session{}, newTotal: 0}
	}
}

func deleteSession(sessionID string, _ *db.Store) tea.Cmd {
	return func() tea.Msg {
		return deletedMsg{sessionID: sessionID, newTotal: 0}
	}
}

func openSession(sessionID string) tea.Cmd {
	return func() tea.Msg {
		return openedMsg{sessionID: sessionID}
	}
}

func pruneOrphans(_ *db.Store) tea.Cmd {
	return func() tea.Msg {
		return prunedMsg{count: 3, bytes: 81_920}
	}
}

func vacuumDB(_ *db.Store, beforeTotal int64) tea.Cmd {
	return func() tea.Msg {
		compacted := beforeTotal / 2
		return vacuumedMsg{dbUsed: compacted, dbTotal: compacted, beforeTotal: beforeTotal}
	}
}

func reloadData(_ *db.Store, _ bool, costPeriod CostPeriod) tea.Cmd {
	return func() tea.Msg {
		msg := reloadedMsg{
			sessions:    []db.Session{},
			newTotal:    0,
			dbUsed:      2_621_440,
			dbTotal:     4_194_304,
			orphanCount: 3,
			orphanBytes: 81_920,
		}
		if costPeriod != "" {
			msg.periodCost = 2.51
		}
		return msg
	}
}
