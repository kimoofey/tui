//go:build mock

package main

import (
	"context"
	"github.com/kimoofey/tui/internal/terminal"
	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
	"sort"
	"sync"
	"time"
)

// The mock preview has in-memory accounting only. No user config, source,
// archive or native executable is consulted.
type mockHistory struct {
	opts     ocm.Options
	browser  *service.HistoryBrowser
	mu       sync.Mutex
	complete bool
}
type mockHistorySource struct {
	mu       sync.Mutex
	sessions []session.Session
}

func (s *mockHistorySource) Info() session.Info {
	return session.Info{ID: "mock", Name: "Mock OpenCode", RootFilter: true}
}
func (s *mockHistorySource) List(context.Context, session.ListOptions) (session.Listing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return session.Listing{Sessions: append([]session.Session(nil), s.sessions...), TotalCount: len(s.sessions)}, nil
}
func (s *mockHistorySource) Close() error { return nil }
func newMockHistory(opts ocm.Options) *mockHistory {
	b := &mockHistory{opts: opts}
	b.browser = service.NewHistoryBrowser(mockImportArchive{b}, func(context.Context) (session.Source, error) { return &mockHistorySource{sessions: opts.Sessions}, nil }, nil, service.LoadOptions{})
	return b
}
func (b *mockHistory) Refresh(ctx context.Context) service.BrowserState {
	state := b.browser.Refresh(ctx)
	if state.Err != nil {
		return state
	}
	return service.BrowserState{App: state.App, ImportPending: state.ImportPending, Snapshot: &service.Snapshot{Sessions: state.Snapshot.Sessions, TotalCount: state.Snapshot.TotalCount, Capabilities: state.Snapshot.Capabilities, Metrics: &session.Metrics{Used: b.opts.DBUsed, Total: b.opts.DBTotal}, PeriodCost: &session.RecordedCost{Amount: b.opts.PeriodCost}, Orphans: &session.Orphans{Count: b.opts.OrphanCount, Bytes: b.opts.OrphanBytes}}}
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

func (b *mockHistory) Import(ctx context.Context) error { return b.browser.Import(ctx) }
func (b *mockHistory) Close() error                     { return b.browser.Close() }

type mockImportArchive struct{ *mockHistory }

func (mockImportArchive) Close() error                                               { return nil }
func (b *mockHistory) Capture(context.Context, session.Observation, time.Time) error { return nil }
func (b *mockHistory) RecordFailure(context.Context, string) error                   { return nil }
func (b *mockHistory) InitialImportComplete(context.Context) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.complete, nil
}
func (b *mockHistory) CompleteInitialImport(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.complete = true
	return nil
}
func (s *mockHistorySource) HistorySessions(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for _, row := range s.sessions {
		ids = append(ids, row.ID)
	}
	return ids, nil
}
func (s *mockHistorySource) CaptureHistory(ctx context.Context, id string) (session.Observation, error) {
	// Keep the in-memory preview long enough to exercise progress and cancellation.
	timer := time.NewTimer(120 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return session.Observation{}, ctx.Err()
	case <-timer.C:
	}
	return session.Observation{Present: true, SessionID: id}, nil
}

// Preview capabilities stay usable after the import updates the service status.
func (s *mockHistorySource) Resume(context.Context, string) (terminal.Command, error) {
	return terminal.Command{}, nil
}
func (s *mockHistorySource) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var remaining []session.Session
	for _, row := range s.sessions {
		if row.ID != id {
			remaining = append(remaining, row)
		}
	}
	s.sessions = remaining
	return nil
}
func (s *mockHistorySource) HistoryDeletionAvailability(context.Context) session.Availability {
	return session.Availability{Available: true}
}
func (s *mockHistorySource) HistoryDeletionScope(_ context.Context, r session.Ref) ([]session.Ref, error) {
	return []session.Ref{r}, nil
}
func (s *mockHistorySource) Metrics(context.Context) (session.Metrics, error) {
	return session.Metrics{Used: 2621440, Total: 4194304}, nil
}
func (s *mockHistorySource) PeriodCost(context.Context, time.Time, time.Time) (session.RecordedCost, error) {
	return session.RecordedCost{Amount: 2.51}, nil
}
func (s *mockHistorySource) VacuumAvailability() session.Availability {
	return session.Availability{Available: true}
}
func (s *mockHistorySource) Vacuum(context.Context) (session.Metrics, error) {
	return session.Metrics{Used: 2621440, Total: 2621440}, nil
}
func (s *mockHistorySource) OrphanAvailability() session.Availability {
	return session.Availability{Available: true}
}
func (s *mockHistorySource) InspectOrphans(context.Context) (session.Orphans, error) {
	return session.Orphans{Count: 3, Bytes: 81920}, nil
}
func (s *mockHistorySource) Prune(context.Context) (session.Orphans, error) {
	return session.Orphans{Count: 3, Bytes: 81920}, nil
}
