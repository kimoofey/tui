//go:build mock

package main

import (
	"context"
	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
	"sort"
)

// The mock preview has in-memory accounting only. No user config, source,
// archive or native executable is consulted.
type mockHistory struct {
	opts ocm.Options
	app  *service.Service
}
type mockHistorySource struct{ sessions []session.Session }

func (s *mockHistorySource) Info() session.Info {
	return session.Info{ID: "mock", Name: "Mock OpenCode"}
}
func (s *mockHistorySource) List(context.Context, session.ListOptions) (session.Listing, error) {
	return session.Listing{Sessions: s.sessions, TotalCount: len(s.sessions)}, nil
}
func (s *mockHistorySource) Close() error { return nil }
func newMockHistory(opts ocm.Options) *mockHistory {
	return &mockHistory{opts: opts, app: service.New(&mockHistorySource{sessions: opts.Sessions}, nil, nil)}
}
func (b *mockHistory) Refresh(context.Context) service.BrowserState {
	return service.BrowserState{App: b.app, Snapshot: &service.Snapshot{Sessions: b.opts.Sessions, TotalCount: b.opts.TotalCount, Capabilities: b.opts.Capabilities, Metrics: &session.Metrics{Used: b.opts.DBUsed, Total: b.opts.DBTotal}, PeriodCost: &session.RecordedCost{Amount: b.opts.PeriodCost}, Orphans: &session.Orphans{Count: b.opts.OrphanCount, Bytes: b.opts.OrphanBytes}}}
}
func (b *mockHistory) Query(_ context.Context, period *history.Range) (history.Report, error) {
	var report history.Report
	days := map[string]*history.Day{}
	total := 0.0
	for _, s := range b.opts.Sessions {
		report.Sessions = append(report.Sessions, history.SessionStatus{ID: s.ID, Captured: mockDataNow.UnixMilli()})
		at := s.Updated.UnixMilli()
		if period != nil && (at < period.Start || at >= period.End) {
			continue
		}
		date := s.Updated.UTC().Format("2006-01-02")
		day := days[date]
		if day == nil {
			zero := 0.0
			day = &history.Day{UTC: date, Total: history.Total{Cost: &zero}}
			days[date] = day
		}
		*day.Cost += s.Cost.Amount
		day.Rows++
		total += s.Cost.Amount
		report.Rows++
	}
	for _, day := range days {
		report.Days = append(report.Days, *day)
	}
	sort.Slice(report.Days, func(i, j int) bool { return report.Days[i].UTC < report.Days[j].UTC })
	if report.Rows > 0 {
		report.Cost = &total
		p, m := "Mock provider", "Mock model"
		report.Models = []history.Model{{Provider: &p, Name: &m, Total: report.Total}}
	}
	return report, nil
}
