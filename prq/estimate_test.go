package prq

import "testing"

func TestEstimateReviewTime_DefaultBucketsGranular(t *testing.T) {
	got := EstimateReviewTime("alice", "fix: nil pointer", 21, 0, 1, defaultEstimateTimeBuckets())
	if got != "~3m" {
		t.Errorf("EstimateReviewTime() = %q; want %q", got, "~3m")
	}
}

func TestEstimateReviewTime_BotShortCircuit(t *testing.T) {
	got := EstimateReviewTime("dependabot[bot]", "chore: bump x", 2000, 1000, 80, defaultEstimateTimeBuckets())
	if got != "~1m" {
		t.Errorf("EstimateReviewTime() = %q; want %q", got, "~1m")
	}
}

func TestEstimateToLabel_CustomBuckets(t *testing.T) {
	got := estimateToLabel(7.9, []int{1, 2, 4, 6, 8, 12})
	if got != "~8m" {
		t.Errorf("estimateToLabel() = %q; want %q", got, "~8m")
	}
}

func TestEstimateToLabel_CustomBucketsTailPlus(t *testing.T) {
	got := estimateToLabel(20.1, []int{1, 2, 4, 6, 8, 12})
	if got != "~12m+" {
		t.Errorf("estimateToLabel() = %q; want %q", got, "~12m+")
	}
}

func TestNormalizedBuckets_DefaultFallback(t *testing.T) {
	got := normalizedBuckets([]int{0, -1})
	want := defaultEstimateTimeBuckets()
	if len(got) != len(want) {
		t.Fatalf("normalizedBuckets len = %d; want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("normalizedBuckets[%d] = %d; want %d", i, got[i], want[i])
		}
	}
}

func TestNormalizedBuckets_SortsAndCompacts(t *testing.T) {
	got := normalizedBuckets([]int{12, 1, 3, 3, 2, -2, 0, 8})
	want := []int{1, 2, 3, 8, 12}
	if len(got) != len(want) {
		t.Fatalf("normalizedBuckets len = %d; want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("normalizedBuckets[%d] = %d; want %d", i, got[i], want[i])
		}
	}
}
