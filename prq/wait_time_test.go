package prq

import (
	"testing"
	"time"
)

func TestSelectWaitSince_DirectUsesLatestMatchingEvent(t *testing.T) {
	fallback := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	node := ghPRNode{
		TimelineItems: struct {
			Nodes []ghReviewRequestedEvent `json:"nodes"`
		}{
			Nodes: []ghReviewRequestedEvent{
				{CreatedAt: "2025-06-02T10:00:00Z", RequestedReviewer: &ghRequestedReviewer{Login: "other"}},
				{CreatedAt: "2025-06-03T10:00:00Z", RequestedReviewer: &ghRequestedReviewer{Login: "me"}},
				{CreatedAt: "2025-06-04T10:00:00Z", RequestedReviewer: &ghRequestedReviewer{Login: "me"}},
			},
		},
	}

	got := selectWaitSince(node, BucketDirect, "me", fallback)
	want := time.Date(2025, 6, 4, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("selectWaitSince() = %v; want %v", got, want)
	}
}

func TestSelectWaitSince_TeamUsesLatestMatchingCurrentRequestedTeam(t *testing.T) {
	fallback := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	node := ghPRNode{
		ReviewRequests: struct {
			Nodes []ghReviewRequest `json:"nodes"`
		}{
			Nodes: []ghReviewRequest{
				{RequestedReviewer: struct {
					Login string `json:"login"`
					Slug  string `json:"slug"`
				}{Slug: "core-team"}},
			},
		},
		TimelineItems: struct {
			Nodes []ghReviewRequestedEvent `json:"nodes"`
		}{
			Nodes: []ghReviewRequestedEvent{
				{CreatedAt: "2025-06-02T09:00:00Z", RequestedReviewer: &ghRequestedReviewer{Slug: "infra-team"}},
				{CreatedAt: "2025-06-03T09:00:00Z", RequestedReviewer: &ghRequestedReviewer{Slug: "core-team"}},
				{CreatedAt: "2025-06-04T09:00:00Z", RequestedReviewer: &ghRequestedReviewer{Slug: "core-team"}},
			},
		},
	}

	got := selectWaitSince(node, BucketTeam, "me", fallback)
	want := time.Date(2025, 6, 4, 9, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("selectWaitSince() = %v; want %v", got, want)
	}
}

func TestSelectWaitSince_WatchUsesLatestReviewRequest(t *testing.T) {
	fallback := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	node := ghPRNode{
		TimelineItems: struct {
			Nodes []ghReviewRequestedEvent `json:"nodes"`
		}{
			Nodes: []ghReviewRequestedEvent{
				{CreatedAt: "2025-06-02T10:00:00Z"},
				{CreatedAt: "2025-06-04T10:00:00Z"},
			},
		},
	}

	got := selectWaitSince(node, BucketWatch, "me", fallback)
	want := time.Date(2025, 6, 4, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("selectWaitSince() = %v; want %v", got, want)
	}
}

func TestSelectWaitSince_WatchFallsBackWhenNoEvents(t *testing.T) {
	fallback := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	node := ghPRNode{}

	got := selectWaitSince(node, BucketWatch, "me", fallback)
	if !got.Equal(fallback) {
		t.Fatalf("selectWaitSince() = %v; want fallback %v", got, fallback)
	}
}

func TestSelectWaitSince_FallbackWhenNoMatch(t *testing.T) {
	fallback := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	node := ghPRNode{
		TimelineItems: struct {
			Nodes []ghReviewRequestedEvent `json:"nodes"`
		}{
			Nodes: []ghReviewRequestedEvent{
				{CreatedAt: "2025-06-03T10:00:00Z", RequestedReviewer: &ghRequestedReviewer{Login: "other"}},
			},
		},
	}

	got := selectWaitSince(node, BucketDirect, "me", fallback)
	if !got.Equal(fallback) {
		t.Fatalf("selectWaitSince() = %v; want fallback %v", got, fallback)
	}
}
