//go:build mock

package prq

import "time"

var mockNow = time.Now()

func FetchAll(_ Config) FetchResult {
	reviewPRs := []PullRequest{
		{
			NodeID:     "R_kgDOAA142",
			Number:    142,
			Title:     "feat: add rate limiting middleware to API gateway",
			URL:       "https://github.com/acme/platform/pull/142",
			HeadRefOID: "sha142",
			Author:    "jsmith",
			Repo:      "acme/platform",
			CreatedAt: mockNow.AddDate(0, 0, -2),
			WaitSince: mockNow.Add(-3 * time.Hour),
			Approvals: 0,
			Bucket:    BucketDirect,
			Enriched:  true,
		},
		{
			NodeID:     "R_kgDOAA138",
			Number:    138,
			Title:     "fix: resolve race condition in session store",
			URL:       "https://github.com/acme/platform/pull/138",
			HeadRefOID: "sha138",
			Author:    "mlopez",
			Repo:      "acme/platform",
			CreatedAt: mockNow.AddDate(0, 0, -4),
			WaitSince: mockNow.AddDate(0, 0, -1),
			Approvals: 1,
			Bucket:    BucketDirect,
			Enriched:  true,
		},
		{
			NodeID:     "R_kgDOAA77",
			Number:    77,
			Title:     "refactor: extract auth logic into separate service",
			URL:       "https://github.com/acme/backend/pull/77",
			HeadRefOID: "sha77",
			Author:    "achen",
			Repo:      "acme/backend",
			CreatedAt: mockNow.AddDate(0, 0, -1),
			WaitSince: mockNow.Add(-90 * time.Minute),
			Approvals: 0,
			Bucket:    BucketTeam,
			Enriched:  true,
		},
		{
			NodeID:     "R_kgDOAA203",
			Number:    203,
			Title:     "chore: bump Go toolchain to 1.24",
			URL:       "https://github.com/acme/infra/pull/203",
			HeadRefOID: "sha203",
			Author:    "renovate[bot]",
			Repo:      "acme/infra",
			CreatedAt: mockNow.AddDate(0, 0, -6),
			WaitSince: mockNow.AddDate(0, 0, -6),
			Approvals: 0,
			Bucket:    BucketWatch,
			Enriched:  true,
		},
		{
			NodeID:     "R_kgDOAA91",
			Number:    91,
			Title:     "feat: add prometheus metrics endpoint",
			URL:       "https://github.com/acme/backend/pull/91",
			HeadRefOID: "sha91",
			Author:    "dpark",
			Repo:      "acme/backend",
			CreatedAt: mockNow.AddDate(0, 0, -3),
			WaitSince: mockNow.AddDate(0, 0, -3),
			Approvals: 1,
			Bucket:    BucketWatch,
			Enriched:  true,
		},
	}

	myPRs := []PullRequest{
		{
			NodeID:         "R_kgDOAA55",
			Number:         55,
			Title:          "feat: implement VHS screenshot automation",
			URL:            "https://github.com/acme/tui/pull/55",
			HeadRefOID:     "sha55",
			Author:         "you",
			Repo:           "acme/tui",
			CreatedAt:      mockNow.AddDate(0, 0, -1),
			Bucket:         BucketMine,
			IsDraft:        false,
			ReviewDecision: "REVIEW_REQUIRED",
		},
		{
			NodeID:         "R_kgDOAA48",
			Number:         48,
			Title:          "wip: spike on bubble tea v2 migration",
			URL:            "https://github.com/acme/tui/pull/48",
			HeadRefOID:     "sha48",
			Author:         "you",
			Repo:           "acme/tui",
			CreatedAt:      mockNow.AddDate(0, 0, -7),
			Bucket:         BucketMine,
			IsDraft:        true,
			ReviewDecision: "",
		},
		{
			NodeID:         "R_kgDOAA312",
			Number:         312,
			Title:          "fix: handle nil pointer in config loader",
			URL:            "https://github.com/acme/platform/pull/312",
			HeadRefOID:     "sha312",
			Author:         "you",
			Repo:           "acme/platform",
			CreatedAt:      mockNow.AddDate(0, 0, -3),
			Bucket:         BucketMine,
			IsDraft:        false,
			ReviewDecision: "APPROVED",
		},
	}

	return FetchResult{ReviewPRs: reviewPRs, MyPRs: myPRs}
}

func EnrichReviewPRs(_ Config, reviewPRs []PullRequest) enrichResult {
	updates := make(map[string]PullRequest, len(reviewPRs))
	for _, pr := range reviewPRs {
		pr.Enriched = true
		updates[pr.URL] = pr
	}
	return enrichResult{Updates: updates}
}

func PrefetchUser() error { return nil }
