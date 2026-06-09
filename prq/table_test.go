package prq

import (
	"testing"
	"time"
)

func TestBucketLabel(t *testing.T) {
	tests := []struct {
		bucket Bucket
		want   string
	}{
		{BucketDirect, "[direct]"},
		{BucketTeam, "[team]  "},
		{BucketWatch, "[watch] "},
		{BucketMine, "[mine]  "},
	}
	for _, tt := range tests {
		got := bucketLabel(tt.bucket)
		if got != tt.want {
			t.Errorf("bucketLabel(%q) = %q; want %q", tt.bucket, got, tt.want)
		}
	}
}

func TestDraftLabel(t *testing.T) {
	if got := draftLabel(true); got != "[draft] " {
		t.Errorf("draftLabel(true) = %q; want %q", got, "[draft] ")
	}
	if got := draftLabel(false); got != "[open]  " {
		t.Errorf("draftLabel(false) = %q; want %q", got, "[open]  ")
	}
}

func TestReviewDecisionLabel(t *testing.T) {
	tests := []struct {
		decision string
		want     string
	}{
		{"APPROVED", "approved"},
		{"CHANGES_REQUESTED", "changes"},
		{"REVIEW_REQUIRED", "pending"},
		{"", "none"},
		{"UNKNOWN_STATE", "none"},
	}
	for _, tt := range tests {
		got := reviewDecisionLabel(tt.decision)
		if got != tt.want {
			t.Errorf("reviewDecisionLabel(%q) = %q; want %q", tt.decision, got, tt.want)
		}
	}
}

func TestPRsToRows(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	prs := []PullRequest{
		{
			Number:    1,
			Title:     "Fix bug",
			Author:    "alice",
			Repo:      "owner/repo",
			CreatedAt: now.Add(-2 * 24 * time.Hour),
			Approvals: 1,
			Bucket:    BucketDirect,
			Enriched:  false,
		},
		{
			Number:    2,
			Title:     "Add feature",
			Author:    "bob",
			Repo:      "owner/other-repo",
			CreatedAt: now.Add(-1 * time.Hour),
			Approvals: 2,
			Bucket:    BucketTeam,
			Enriched:  true,
		},
	}

	rows := PRsToRows(prs, defaultEstimateTimeBuckets())

	if len(rows) != 2 {
		t.Fatalf("PRsToRows: got %d rows; want 2", len(rows))
	}

	// Row 0: bucket, repo, title, author, age, estimate, review_decision
	if rows[0][0] != "[direct]" {
		t.Errorf("row[0] source = %q; want %q", rows[0][0], "[direct]")
	}
	if rows[0][4] != "..." {
		t.Errorf("row[0] pending = %q; want %q", rows[0][4], "...")
	}
	if rows[0][5] != "..." {
		t.Errorf("row[0] estimate = %q; want %q", rows[0][5], "...")
	}
	if rows[0][6] != "none" {
		t.Errorf("row[0] decision = %q; want %q", rows[0][6], "none")
	}
}

func TestMyPRsToRows(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	prs := []PullRequest{
		{
			Title:          "My draft PR",
			Repo:           "owner/repo",
			CreatedAt:      now.Add(-5 * time.Hour),
			IsDraft:        true,
			ReviewDecision: "REVIEW_REQUIRED",
		},
		{
			Title:          "My open PR",
			Repo:           "owner/repo",
			CreatedAt:      now.Add(-1 * 24 * time.Hour),
			IsDraft:        false,
			ReviewDecision: "APPROVED",
		},
	}

	rows := MyPRsToRows(prs)

	if len(rows) != 2 {
		t.Fatalf("MyPRsToRows: got %d rows; want 2", len(rows))
	}
	if rows[0][0] != "[draft] " {
		t.Errorf("row[0] status = %q; want %q", rows[0][0], "[draft] ")
	}
	if rows[1][0] != "[open]  " {
		t.Errorf("row[1] status = %q; want %q", rows[1][0], "[open]  ")
	}
	if rows[1][4] != "approved" {
		t.Errorf("row[1] review = %q; want %q", rows[1][4], "approved")
	}
}
