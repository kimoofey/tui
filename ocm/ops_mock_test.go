//go:build mock

package ocm

import (
	"testing"

	"github.com/kimoofey/tui/ocm/db"
)

func TestMockDeletePreservesRemainingRowsAndStats(t *testing.T) {
	model := New(Options{
		Sessions: []db.Session{{ID: "one"}, {ID: "two"}}, TotalCount: 2,
		DBUsed: 100, DBTotal: 200, PeriodCost: 3,
	}, 100, 30)
	updated, _ := model.Update(deleteSession("one", nil, false, "")())
	got := updated.(Model)
	if len(got.sessions) != 1 || got.sessions[0].ID != "two" || got.totalSessions != 1 || got.dbTotal != 200 || got.periodCost != 3 {
		t.Fatalf("mock deletion changed remaining data: sessions=%v, total=%d, db=%d, cost=%v", got.sessions, got.totalSessions, got.dbTotal, got.periodCost)
	}
}
