//go:build mock

package prq

import "time"

var mockNow = time.Now()

func FetchAll(_ Config) FetchResult {
	reviewPRs := []PullRequest{
		{
			Number:    142,
			Title:     "feat: add rate limiting middleware to API gateway",
			URL:       "https://github.com/acme/platform/pull/142",
			Author:    "jsmith",
			Repo:      "acme/platform",
			CreatedAt: mockNow.AddDate(0, 0, -2),
			Approvals: 0,
			Bucket:    BucketDirect,
		},
		{
			Number:    138,
			Title:     "fix: resolve race condition in session store",
			URL:       "https://github.com/acme/platform/pull/138",
			Author:    "mlopez",
			Repo:      "acme/platform",
			CreatedAt: mockNow.AddDate(0, 0, -4),
			Approvals: 1,
			Bucket:    BucketDirect,
		},
		{
			Number:    77,
			Title:     "refactor: extract auth logic into separate service",
			URL:       "https://github.com/acme/backend/pull/77",
			Author:    "achen",
			Repo:      "acme/backend",
			CreatedAt: mockNow.AddDate(0, 0, -1),
			Approvals: 0,
			Bucket:    BucketTeam,
		},
		{
			Number:    203,
			Title:     "chore: bump Go toolchain to 1.24",
			URL:       "https://github.com/acme/infra/pull/203",
			Author:    "renovate[bot]",
			Repo:      "acme/infra",
			CreatedAt: mockNow.AddDate(0, 0, -6),
			Approvals: 0,
			Bucket:    BucketWatch,
		},
		{
			Number:    91,
			Title:     "feat: add prometheus metrics endpoint",
			URL:       "https://github.com/acme/backend/pull/91",
			Author:    "dpark",
			Repo:      "acme/backend",
			CreatedAt: mockNow.AddDate(0, 0, -3),
			Approvals: 1,
			Bucket:    BucketWatch,
		},
	}

	myPRs := []PullRequest{
		{
			Number:         55,
			Title:          "feat: implement VHS screenshot automation",
			URL:            "https://github.com/acme/tui/pull/55",
			Author:         "you",
			Repo:           "acme/tui",
			CreatedAt:      mockNow.AddDate(0, 0, -1),
			Bucket:         BucketMine,
			IsDraft:        false,
			ReviewDecision: "REVIEW_REQUIRED",
		},
		{
			Number:         48,
			Title:          "wip: spike on bubble tea v2 migration",
			URL:            "https://github.com/acme/tui/pull/48",
			Author:         "you",
			Repo:           "acme/tui",
			CreatedAt:      mockNow.AddDate(0, 0, -7),
			Bucket:         BucketMine,
			IsDraft:        true,
			ReviewDecision: "",
		},
		{
			Number:         312,
			Title:          "fix: handle nil pointer in config loader",
			URL:            "https://github.com/acme/platform/pull/312",
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
