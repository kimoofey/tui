package ocm

import (
	"errors"
	"testing"

	"github.com/kimoofey/tui/ocm/db"
)

func TestBulkDelete_ExplicitEmptyReloadIsAuthoritative(t *testing.T) {
	model := New(Options{
		Sessions: []db.Session{{ID: "one"}}, TotalCount: 1,
		DBUsed: 100, DBTotal: 200, PeriodCost: 3, OrphanCount: 4, OrphanBytes: 500,
	}, 100, 30)
	updated, _ := model.Update(bulkDeletedMsg{count: 1, data: &reloadedMsg{}})
	got := updated.(Model)
	if len(got.sessions) != 0 || got.totalSessions != 0 || got.dbUsed != 0 || got.dbTotal != 0 || got.periodCost != 0 || got.orphanCount != 0 || got.orphanBytes != 0 {
		t.Fatal("explicit empty snapshot was treated as an acknowledgement")
	}
}

func TestBulkDelete_FailureWithoutValidReloadPreservesData(t *testing.T) {
	err := errors.New("delete or reload failed")
	for _, data := range []*reloadedMsg{nil, {err: err}} {
		model := New(Options{
			Sessions: []db.Session{{ID: "one"}}, TotalCount: 1,
			DBUsed: 100, DBTotal: 200, PeriodCost: 3, OrphanCount: 4, OrphanBytes: 500,
		}, 100, 30)
		updated, _ := model.Update(bulkDeletedMsg{ids: []string{"one"}, data: data, err: err})
		got := updated.(Model)
		if len(got.sessions) != 1 || got.totalSessions != 1 || got.dbUsed != 100 || got.dbTotal != 200 || got.periodCost != 3 || got.orphanCount != 4 || got.orphanBytes != 500 || got.statusOK {
			t.Fatal("failed delete/reload changed the displayed data")
		}
	}
}
