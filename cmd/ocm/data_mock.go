//go:build mock

package main

import (
	"time"

	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/db"
)

var mockDataNow = time.Now()

func loadData(sessionsOnly bool) (ocm.Options, error) {
	sessions := []db.Session{
		{
			ID:        "aabbcc11-0001-0001-0001-000000000001",
			Title:     "Refactor authentication middleware",
			Directory: "~/projects/platform",
			Created:   mockDataNow.Add(-72 * time.Hour),
			Updated:   mockDataNow.Add(-10 * time.Minute),
			Cost:      0.42,
		},
		{
			ID:        "aabbcc11-0002-0002-0002-000000000002",
			Title:     "Fix race condition in session store",
			Directory: "~/projects/platform",
			Created:   mockDataNow.Add(-48 * time.Hour),
			Updated:   mockDataNow.Add(-2 * time.Hour),
			Cost:      0.18,
		},
		{
			ID:        "aabbcc11-0003-0003-0003-000000000003",
			Title:     "Add Prometheus metrics endpoint",
			Directory: "~/projects/backend",
			Created:   mockDataNow.Add(-36 * time.Hour),
			Updated:   mockDataNow.Add(-5 * time.Hour),
			Cost:      0.31,
		},
		{
			ID:        "aabbcc11-0004-0004-0004-000000000004",
			Title:     "Bump Go toolchain to 1.24",
			Directory: "~/projects/infra",
			Created:   mockDataNow.Add(-24 * time.Hour),
			Updated:   mockDataNow.Add(-8 * time.Hour),
			Cost:      0.07,
		},
		{
			ID:        "aabbcc11-0005-0005-0005-000000000005",
			Title:     "Implement VHS screenshot automation",
			Directory: "~/projects/tui",
			Created:   mockDataNow.Add(-12 * time.Hour),
			Updated:   mockDataNow.Add(-30 * time.Minute),
			Cost:      0.55,
		},
		{
			ID:        "aabbcc11-0006-0006-0006-000000000006",
			Title:     "Spike: Bubble Tea v2 migration",
			Directory: "~/projects/tui",
			Created:   mockDataNow.Add(-96 * time.Hour),
			Updated:   mockDataNow.Add(-24 * time.Hour),
			Cost:      0.23,
		},
		{
			ID:        "aabbcc11-0007-0007-0007-000000000007",
			Title:     "Extract rate limiting into shared lib",
			Directory: "~/projects/backend",
			Created:   mockDataNow.Add(-120 * time.Hour),
			Updated:   mockDataNow.Add(-48 * time.Hour),
			Cost:      0.61,
		},
		{
			ID:        "aabbcc11-0008-0008-0008-000000000008",
			Title:     "Update OpenAPI spec for v3 endpoints",
			Directory: "~/projects/api-docs",
			Created:   mockDataNow.Add(-168 * time.Hour),
			Updated:   mockDataNow.Add(-72 * time.Hour),
			Cost:      0.14,
		},
	}

	return ocm.Options{
		Sessions:    sessions,
		TotalCount:  len(sessions),
		DBUsed:      2_621_440,
		DBTotal:     4_194_304,
		OrphanCount: 3,
		OrphanBytes: 81_920,
		DBPath:      "/mock/opencode.db",
		RootOnly:    sessionsOnly,
	}, nil
}
