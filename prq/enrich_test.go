package prq

import (
	"testing"
	"time"
)

func TestGroupReviewPRs_DirectTeamWatchOrder(t *testing.T) {
	prs := []PullRequest{
		{URL: "u1", Bucket: BucketWatch},
		{URL: "u2", Bucket: BucketTeam},
		{URL: "u3", Bucket: BucketDirect},
		{URL: "u4", Bucket: BucketWatch},
		{URL: "u5", Bucket: BucketTeam},
	}

	got := groupReviewPRs(prs)
	if len(got) != len(prs) {
		t.Fatalf("groupReviewPRs len=%d; want %d", len(got), len(prs))
	}
	want := []string{"u3", "u2", "u5", "u1", "u4"}
	for i := range want {
		if got[i].URL != want[i] {
			t.Fatalf("groupReviewPRs[%d]=%q; want %q", i, got[i].URL, want[i])
		}
	}
}

func TestMergeEnrichment_UpdatesInPlaceWithoutReorder(t *testing.T) {
	now := time.Now()
	base := []PullRequest{
		{URL: "u1", Bucket: BucketDirect, CreatedAt: now.Add(-3 * time.Hour)},
		{URL: "u2", Bucket: BucketTeam, CreatedAt: now.Add(-2 * time.Hour)},
		{URL: "u3", Bucket: BucketWatch, CreatedAt: now.Add(-1 * time.Hour)},
	}
	updates := map[string]PullRequest{
		"u2": {
			URL:            "u2",
			WaitSince:      now.Add(-90 * time.Minute),
			Files:          []PRFile{{Path: "a.go", Additions: 10, Deletions: 2}},
			FilesTruncated: false,
		},
		"u3": {
			URL:            "u3",
			WaitSince:      now.Add(-45 * time.Minute),
			Files:          []PRFile{{Path: "b.go", Additions: 4, Deletions: 1}},
			FilesTruncated: true,
		},
	}

	merged, count := mergeEnrichment(base, updates)
	if count != 2 {
		t.Fatalf("mergeEnrichment count=%d; want 2", count)
	}
	for i, wantURL := range []string{"u1", "u2", "u3"} {
		if merged[i].URL != wantURL {
			t.Fatalf("merged[%d].URL=%q; want %q", i, merged[i].URL, wantURL)
		}
	}
	if merged[0].Enriched {
		t.Fatalf("merged[0].Enriched=true; want false")
	}
	if !merged[1].Enriched || !merged[2].Enriched {
		t.Fatalf("expected u2 and u3 to be enriched")
	}
	if len(merged[1].Files) != 1 || merged[1].Files[0].Path != "a.go" {
		t.Fatalf("u2 files not updated")
	}
	if !merged[2].FilesTruncated {
		t.Fatalf("u3 FilesTruncated=false; want true")
	}
}
