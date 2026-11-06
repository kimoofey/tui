//go:build mock

package prq

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

func TestMockFetchFilters(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		configure func(*Config)
		want      []int
	}{
		{name: "defaults", want: []int{142, 77}},
		{name: "include reviewed", configure: func(c *Config) { c.SkipAlreadyReviewed = false }, want: []int{142, 138, 77}},
		{name: "watch repo still excludes approved", configure: func(c *Config) { c.WatchRepos = []string{"acme/backend"} }, want: []int{142, 77}},
		{name: "include approved from watched repo", configure: func(c *Config) {
			c.WatchRepos = []string{"acme/backend"}
			c.SkipApproved = false
		}, want: []int{142, 77, 91}},
		{name: "watch repo still excludes bots", configure: func(c *Config) { c.WatchRepos = []string{"acme/infra"} }, want: []int{142, 77}},
		{name: "include bots from watched repo", configure: func(c *Config) {
			c.WatchRepos = []string{"acme/infra"}
			c.SkipBots = false
		}, want: []int{142, 77, 203}},
		{name: "unrelated watch repo", configure: func(c *Config) {
			c.WatchRepos = []string{"other/repo"}
			c.SkipBots = false
			c.SkipApproved = false
		}, want: []int{142, 77}},
		{name: "one day including boundary date", configure: func(c *Config) { c.DaysAgo = 1 }, want: []int{77}},
		{name: "two days including boundary date", configure: func(c *Config) { c.DaysAgo = 2 }, want: []int{142, 77}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			if tt.configure != nil {
				tt.configure(&cfg)
			}
			result := fetchMockAt(context.Background(), cfg, now)
			if result.Err != nil {
				t.Fatal(result.Err)
			}
			var numbers []int
			for _, pr := range result.ReviewPRs {
				numbers = append(numbers, pr.Number)
			}
			if !slices.Equal(numbers, tt.want) {
				t.Fatalf("review PRs = %v; want %v", numbers, tt.want)
			}
			// Authored PRs retain drafts, approved PRs, and older PRs.
			if len(result.MyPRs) != 3 {
				t.Fatalf("My PRs = %d; want 3", len(result.MyPRs))
			}
		})
	}
}

func TestMockFixtureRows(t *testing.T) {
	cfg := defaultConfig()
	cfg.WatchRepos = []string{"acme/backend", "acme/infra"}
	cfg.SkipBots, cfg.SkipAlreadyReviewed, cfg.SkipApproved = false, false, false
	result := FetchAll(context.Background(), cfg)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if len(result.ReviewPRs) != 5 {
		t.Fatalf("review PRs = %d; want 5", len(result.ReviewPRs))
	}
	wantStatuses := []string{"review", "commented", "review", "review", "settled"}
	estimates := make(map[string]bool)
	for i, pr := range result.ReviewPRs {
		if !pr.Enriched || len(pr.Files) == 0 || pr.ChangedFiles != len(pr.Files) {
			t.Fatalf("PR #%d lacks complete file stats", pr.Number)
		}
		additions, deletions := 0, 0
		for _, file := range pr.Files {
			additions += file.Additions
			deletions += file.Deletions
		}
		if pr.Additions != additions || pr.Deletions != deletions {
			t.Fatalf("PR #%d has inconsistent diff totals", pr.Number)
		}
		if got := reviewQueueStatusLabel(pr); got != wantStatuses[i] {
			t.Errorf("PR #%d status = %q; want %q", pr.Number, got, wantStatuses[i])
		}
		estimate := estimateCell(pr, cfg.EstimateTimeBuckets)
		if estimate == "..." || estimate == "~2m" {
			t.Errorf("PR #%d has placeholder estimate %q", pr.Number, estimate)
		}
		estimates[estimate] = true
	}
	if len(estimates) < 3 {
		t.Errorf("demo should show varied estimates; got %v", estimates)
	}
	// Mutating one fetch must not corrupt later refreshes or their file stats.
	result.ReviewPRs[0].Files[0].Additions = 0
	refreshed := FetchAll(context.Background(), cfg)
	if refreshed.ReviewPRs[0].Files[0].Additions == 0 {
		t.Error("fetches share mutable fixture data")
	}
	updates := EnrichReviewPRs(context.Background(), cfg, refreshed.ReviewPRs)
	merged, count := mergeEnrichment(refreshed.ReviewPRs, updates.Updates)
	if updates.Err != nil || count != 5 || len(merged[0].Files) == 0 {
		t.Fatalf("mock enrichment lost fixture data: count=%d err=%v", count, updates.Err)
	}
}

func TestMockCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result := FetchAll(ctx, defaultConfig()); !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("fetch error = %v; want canceled", result.Err)
	}
	if result := EnrichReviewPRs(ctx, defaultConfig(), nil); !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("enrich error = %v; want canceled", result.Err)
	}
}
