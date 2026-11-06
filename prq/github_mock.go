//go:build mock

package prq

import (
	"context"
	"slices"
	"strings"
	"time"
)

func FetchAll(ctx context.Context, cfg Config) FetchResult {
	return fetchMockAt(ctx, cfg, time.Now())
}

func fetchMockAt(ctx context.Context, cfg Config, mockNow time.Time) FetchResult {
	if err := ctx.Err(); err != nil {
		return FetchResult{Err: err}
	}
	reviewPRs := []PullRequest{
		{
			NodeID:     "R_kgDOAA142",
			Number:     142,
			Title:      "feat: add rate limiting middleware to API gateway",
			URL:        "https://github.com/acme/platform/pull/142",
			HeadRefOID: "sha142",
			Author:     "jsmith",
			Repo:       "acme/platform",
			CreatedAt:  mockNow.AddDate(0, 0, -2),
			WaitSince:  mockNow.Add(-3 * time.Hour),
			Approvals:  0,
			Bucket:     BucketDirect,
			Enriched:   true,
			Files: []PRFile{
				{Path: "gateway/ratelimit.go", Additions: 180, Deletions: 12},
				{Path: "gateway/ratelimit_test.go", Additions: 110},
				{Path: "docs/api.md", Additions: 20},
			},
		},
		{
			NodeID:                "R_kgDOAA138",
			Number:                138,
			Title:                 "fix: resolve race condition in session store",
			URL:                   "https://github.com/acme/platform/pull/138",
			HeadRefOID:            "sha138",
			Author:                "mlopez",
			Repo:                  "acme/platform",
			CreatedAt:             mockNow.AddDate(0, 0, -4),
			WaitSince:             mockNow.AddDate(0, 0, -1),
			Approvals:             1,
			Bucket:                BucketDirect,
			Enriched:              true,
			ViewerReviewState:     "COMMENTED",
			ViewerReviewAt:        mockNow.Add(-12 * time.Hour),
			LatestReviewRequestAt: mockNow.AddDate(0, 0, -1),
			Files: []PRFile{
				{Path: "session/store.go", Additions: 18, Deletions: 9},
				{Path: "session/store_test.go", Additions: 32},
			},
		},
		{
			NodeID:                "R_kgDOAA77",
			Number:                77,
			Title:                 "refactor: extract auth logic into separate service",
			URL:                   "https://github.com/acme/backend/pull/77",
			HeadRefOID:            "sha77",
			Author:                "achen",
			Repo:                  "acme/backend",
			CreatedAt:             mockNow.AddDate(0, 0, -1),
			WaitSince:             mockNow.Add(-90 * time.Minute),
			Approvals:             0,
			Bucket:                BucketTeam,
			Enriched:              true,
			ViewerReviewState:     "CHANGES_REQUESTED",
			ViewerReviewAt:        mockNow.Add(-4 * time.Hour),
			LatestReviewRequestAt: mockNow.Add(-90 * time.Minute),
			Files: []PRFile{
				{Path: "auth/service.go", Additions: 420, Deletions: 210},
				{Path: "auth/service_test.go", Additions: 180, Deletions: 60},
				{Path: "api/handlers.go", Additions: 35, Deletions: 160},
				{Path: "config/auth.yaml", Additions: 12, Deletions: 4},
			},
		},
		{
			NodeID:     "R_kgDOAA203",
			Number:     203,
			Title:      "chore: bump Go toolchain to 1.24",
			URL:        "https://github.com/acme/infra/pull/203",
			HeadRefOID: "sha203",
			Author:     "renovate[bot]",
			Repo:       "acme/infra",
			CreatedAt:  mockNow.AddDate(0, 0, -6),
			WaitSince:  mockNow.AddDate(0, 0, -6),
			Approvals:  0,
			Bucket:     BucketWatch,
			Enriched:   true,
			Files: []PRFile{
				{Path: "go.mod", Additions: 1, Deletions: 1},
				{Path: "go.sum", Additions: 8, Deletions: 8},
			},
		},
		{
			NodeID:         "R_kgDOAA91",
			Number:         91,
			Title:          "feat: add prometheus metrics endpoint",
			URL:            "https://github.com/acme/backend/pull/91",
			HeadRefOID:     "sha91",
			Author:         "dpark",
			Repo:           "acme/backend",
			CreatedAt:      mockNow.AddDate(0, 0, -3),
			WaitSince:      mockNow.AddDate(0, 0, -3),
			Approvals:      1,
			Bucket:         BucketWatch,
			Enriched:       true,
			ReviewDecision: "APPROVED",
			Files: []PRFile{
				{Path: "metrics/handler.go", Additions: 65, Deletions: 4},
				{Path: "metrics/handler_test.go", Additions: 48},
			},
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

	// GitHub's created:>= filter uses a calendar date. My PRs has no date
	// or review filters in the live implementation, so leave that tab intact.
	date := mockNow.AddDate(0, 0, -cfg.DaysAgo)
	threshold := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	filtered := make([]PullRequest, 0, len(reviewPRs))
	for _, pr := range reviewPRs {
		if pr.CreatedAt.Before(threshold) ||
			(cfg.SkipBots && strings.HasSuffix(pr.Author, "[bot]")) ||
			(cfg.SkipApproved && pr.ReviewDecision == "APPROVED") ||
			(cfg.SkipAlreadyReviewed && pr.ViewerReviewState != "" && !pr.LatestReviewRequestAt.After(pr.ViewerReviewAt)) ||
			(pr.Bucket == BucketWatch && !slices.Contains(cfg.WatchRepos, pr.Repo)) {
			continue
		}
		pr.ChangedFiles = len(pr.Files)
		for _, file := range pr.Files {
			pr.Additions += file.Additions
			pr.Deletions += file.Deletions
		}
		filtered = append(filtered, pr)
	}
	return FetchResult{ReviewPRs: filtered, MyPRs: myPRs}
}

func EnrichReviewPRs(ctx context.Context, _ Config, reviewPRs []PullRequest) enrichResult {
	if err := ctx.Err(); err != nil {
		return enrichResult{Err: err}
	}
	updates := make(map[string]PullRequest, len(reviewPRs))
	for _, pr := range reviewPRs {
		pr.Enriched = true
		updates[pr.URL] = pr
	}
	return enrichResult{Updates: updates}
}

func PrefetchUser() error { return nil }
