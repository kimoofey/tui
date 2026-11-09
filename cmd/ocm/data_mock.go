//go:build mock

package main

import (
	"time"

	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

var mockDataNow = time.Now()

func loadData(sessionsOnly bool, costPeriod service.CostPeriod) (ocm.Options, error) {
	sessions := []session.Session{
		{
			Ref:       session.Ref{Source: "mock", ID: "aabbcc11-0001-0001-0001-000000000001"},
			Title:     "Refactor authentication middleware",
			Directory: "~/projects/platform",
			Created:   mockDataNow.Add(-72 * time.Hour),
			Updated:   mockDataNow.Add(-10 * time.Minute),
			Cost:      &session.RecordedCost{Amount: 0.42, Currency: "USD", Provenance: "source-recorded"},
		},
		{
			Ref:       session.Ref{Source: "mock", ID: "aabbcc11-0002-0002-0002-000000000002"},
			Title:     "Fix race condition in session store",
			Directory: "~/projects/platform",
			Created:   mockDataNow.Add(-48 * time.Hour),
			Updated:   mockDataNow.Add(-2 * time.Hour),
			Cost:      &session.RecordedCost{Amount: 0.18, Currency: "USD", Provenance: "source-recorded"},
		},
		{
			Ref:       session.Ref{Source: "mock", ID: "aabbcc11-0003-0003-0003-000000000003"},
			Title:     "Add Prometheus metrics endpoint",
			Directory: "~/projects/backend",
			Created:   mockDataNow.Add(-36 * time.Hour),
			Updated:   mockDataNow.Add(-5 * time.Hour),
			Cost:      &session.RecordedCost{Amount: 0.31, Currency: "USD", Provenance: "source-recorded"},
		},
		{
			Ref:       session.Ref{Source: "mock", ID: "aabbcc11-0004-0004-0004-000000000004"},
			Title:     "Bump Go toolchain to 1.24",
			Directory: "~/projects/infra",
			Created:   mockDataNow.Add(-24 * time.Hour),
			Updated:   mockDataNow.Add(-8 * time.Hour),
			Cost:      &session.RecordedCost{Amount: 0.07, Currency: "USD", Provenance: "source-recorded"},
		},
		{
			Ref:       session.Ref{Source: "mock", ID: "aabbcc11-0005-0005-0005-000000000005"},
			Title:     "Implement VHS screenshot automation",
			Directory: "~/projects/tui",
			Created:   mockDataNow.Add(-12 * time.Hour),
			Updated:   mockDataNow.Add(-30 * time.Minute),
			Cost:      &session.RecordedCost{Amount: 0.55, Currency: "USD", Provenance: "source-recorded"},
		},
		{
			Ref:       session.Ref{Source: "mock", ID: "aabbcc11-0006-0006-0006-000000000006"},
			Title:     "Spike: Bubble Tea v2 migration",
			Directory: "~/projects/tui",
			Created:   mockDataNow.Add(-96 * time.Hour),
			Updated:   mockDataNow.Add(-24 * time.Hour),
			Cost:      &session.RecordedCost{Amount: 0.23, Currency: "USD", Provenance: "source-recorded"},
		},
		{
			Ref:       session.Ref{Source: "mock", ID: "aabbcc11-0007-0007-0007-000000000007"},
			Title:     "Extract rate limiting into shared lib",
			Directory: "~/projects/backend",
			Created:   mockDataNow.Add(-120 * time.Hour),
			Updated:   mockDataNow.Add(-48 * time.Hour),
			Cost:      &session.RecordedCost{Amount: 0.61, Currency: "USD", Provenance: "source-recorded"},
		},
		{
			Ref:       session.Ref{Source: "mock", ID: "aabbcc11-0008-0008-0008-000000000008"},
			Title:     "Update OpenAPI spec for v3 endpoints",
			Directory: "~/projects/api-docs",
			Created:   mockDataNow.Add(-168 * time.Hour),
			Updated:   mockDataNow.Add(-72 * time.Hour),
			Cost:      &session.RecordedCost{Amount: 0.14, Currency: "USD", Provenance: "source-recorded"},
		},
	}

	opts := ocm.Options{
		Sessions:     sessions,
		TotalCount:   len(sessions),
		DBUsed:       2_621_440,
		DBTotal:      4_194_304,
		CostPeriod:   costPeriod,
		PeriodCost:   2.51,
		OrphanCount:  3,
		OrphanBytes:  81_920,
		Service:      nil,
		MetricsKnown: true,
		Capabilities: service.Capabilities{Resume: true, Delete: true, RootFilter: true, Metrics: true, PeriodCost: true, Vacuum: session.Availability{Available: true}, Orphans: session.Availability{Available: true}},
		RootOnly:     sessionsOnly,
	}
	opts.History = newMockHistory(opts)
	return opts, nil
}
