//go:build mock

package ocm

import (
	"testing"

	"github.com/kimoofey/tui/ocm/session"
)

func TestMockDeletePreservesRemainingRowsAndStats(t *testing.T) {
	model := New(Options{
		Sessions: []session.Session{{Ref: session.Ref{ID: "one"}}, {Ref: session.Ref{ID: "two"}}}, TotalCount: 2,
		DBUsed: 100, DBTotal: 200, PeriodCost: 3,
	}, 100, 30)
	updated, _ := model.Update(deleteSession(session.Ref{ID: "one"}, nil, false, "")())
	got := updated.(Model)
	if len(got.sessions) != 1 || got.sessions[0].ID != "two" || got.totalSessions != 1 || got.dbTotal != 200 || got.periodCost != 3 {
		t.Fatalf("mock deletion changed remaining data: sessions=%v, total=%d, db=%d, cost=%v", got.sessions, got.totalSessions, got.dbTotal, got.periodCost)
	}
}

func TestMockBulkDeletePreservesUnrelatedData(t *testing.T) {
	for _, tt := range []struct {
		name string
		ids  []string
		want []string
	}{
		{"subset", []string{"one", "three"}, []string{"two"}},
		{"all", []string{"one", "two", "three"}, nil},
		{"duplicates and missing IDs", []string{"one", "one", "missing"}, []string{"two", "three"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			model := New(Options{
				Sessions: []session.Session{{Ref: session.Ref{ID: "one"}}, {Ref: session.Ref{ID: "two"}}, {Ref: session.Ref{ID: "three"}}}, TotalCount: 3,
				DBUsed: 100, DBTotal: 200, PeriodCost: 3, OrphanCount: 4, OrphanBytes: 500,
			}, 100, 30)
			model.table.SetCursor(2)
			model.bulkDeleting = true
			model.selected[session.Ref{ID: "one"}] = true
			updated, _ := model.Update(bulkDeleteSessions(refs(tt.ids), nil, false, "")())
			got := updated.(Model)
			if len(got.sessions) != len(tt.want) || got.totalSessions != len(tt.want) {
				t.Fatalf("remaining sessions=%v, count=%d, want=%v", got.sessions, got.totalSessions, tt.want)
			}
			for i, id := range tt.want {
				if got.sessions[i].ID != id {
					t.Fatalf("remaining sessions=%v, want=%v", got.sessions, tt.want)
				}
			}
			if got.dbUsed != 100 || got.dbTotal != 200 || got.periodCost != 3 || got.orphanCount != 4 || got.orphanBytes != 500 {
				t.Fatalf("mock bulk deletion cleared statistics: db=%d/%d, cost=%v, orphans=%d/%d", got.dbUsed, got.dbTotal, got.periodCost, got.orphanCount, got.orphanBytes)
			}
			if got.bulkDeleting || len(got.selected) != 0 || !got.statusOK || got.table.Cursor() > max(len(got.sessions)-1, 0) {
				t.Fatalf("invalid state after bulk acknowledgement: cursor=%d, selected=%v, status=%q", got.table.Cursor(), got.selected, got.status)
			}
		})
	}
}

func refs(ids []string) []session.Ref {
	result := make([]session.Ref, len(ids))
	for i, id := range ids {
		result[i] = session.Ref{ID: id}
	}
	return result
}
