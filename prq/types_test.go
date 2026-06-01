package prq

import (
	"testing"
	"time"
)

func TestAge(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		createdAt time.Time
		want      string
	}{
		{"same day", now.Add(-1 * time.Hour), "today"},
		{"23 hours ago still today", now.Add(-23 * time.Hour), "today"},
		{"exactly 1 day", now.Add(-24 * time.Hour), "1d ago"},
		{"1 day and some hours", now.Add(-30 * time.Hour), "1d ago"},
		{"2 days", now.Add(-2 * 24 * time.Hour), "2d ago"},
		{"30 days", now.Add(-30 * 24 * time.Hour), "30d ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PullRequest{CreatedAt: tt.createdAt}
			got := pr.age(now)
			if got != tt.want {
				t.Errorf("age() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestRepoShort(t *testing.T) {
	tests := []struct {
		repo string
		want string
	}{
		{"owner/my-repo", "my-repo"},
		{"my-repo", "my-repo"},
		{"", ""},
		{"a/b/c", "c"},
		{"owner/", ""},
	}
	for _, tt := range tests {
		pr := PullRequest{Repo: tt.repo}
		got := pr.RepoShort()
		if got != tt.want {
			t.Errorf("RepoShort(%q) = %q; want %q", tt.repo, got, tt.want)
		}
	}
}
