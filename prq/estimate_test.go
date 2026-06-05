package prq

import "testing"

func TestEstimateReviewTime_DefaultBucketsGranular(t *testing.T) {
	got := EstimateReviewTime("alice", "fix: nil pointer", 21, 0, 1, nil, false, defaultEstimateTimeBuckets())
	if got != "~3m" {
		t.Errorf("EstimateReviewTime() = %q; want %q", got, "~3m")
	}
}

func TestEstimateReviewTime_BotShortCircuit(t *testing.T) {
	got := EstimateReviewTime("dependabot[bot]", "chore: bump x", 2000, 1000, 80, nil, false, defaultEstimateTimeBuckets())
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

func TestEstimateReviewTime_MixedFrontendPR(t *testing.T) {
	files := []PRFile{
		{Path: "assets/hero-a.webp", Additions: 0, Deletions: 0},
		{Path: "assets/hero-b.webp", Additions: 0, Deletions: 0},
		{Path: "src/shared/flags.ts", Additions: 1, Deletions: 0},
		{Path: "src/module/view/__tests__/view.spec.tsx", Additions: 11, Deletions: 0},
		{Path: "src/module/view/page.tsx", Additions: 20, Deletions: 6},
		{Path: "src/module/api/client.ts", Additions: 16, Deletions: 0},
		{Path: "src/module/ui/__tests__/card-a.spec.tsx", Additions: 78, Deletions: 0},
		{Path: "src/module/ui/__tests__/card-b.spec.tsx", Additions: 69, Deletions: 0},
		{Path: "src/module/ui/__tests__/card-c.spec.tsx", Additions: 148, Deletions: 0},
		{Path: "src/module/ui/card-a.tsx", Additions: 76, Deletions: 0},
		{Path: "src/module/ui/card-b.tsx", Additions: 122, Deletions: 0},
		{Path: "src/module/ui/card-c.tsx", Additions: 40, Deletions: 0},
		{Path: "src/module/constants.ts", Additions: 6, Deletions: 0},
		{Path: "src/module/hooks/__tests__/use-module.spec.tsx", Additions: 138, Deletions: 0},
		{Path: "src/module/hooks/use-module.ts", Additions: 29, Deletions: 0},
		{Path: "src/module/types.ts", Additions: 22, Deletions: 0},
	}
	got := EstimateReviewTime("alice", "feat: add feature cards", 776, 6, 16, files, false, defaultEstimateTimeBuckets())
	if got != "~30m" {
		t.Errorf("EstimateReviewTime() = %q; want %q", got, "~30m")
	}
}

func TestEstimateReviewTime_BinaryOnlyPR(t *testing.T) {
	files := []PRFile{{Path: "assets/screenshot.webp", Additions: 0, Deletions: 0}}
	got := EstimateReviewTime("alice", "docs: update screenshot", 0, 0, 1, files, false, defaultEstimateTimeBuckets())
	if got != "~1m" {
		t.Errorf("EstimateReviewTime() = %q; want %q", got, "~1m")
	}
}

func TestEstimateReviewTime_TruncatedFallsBackWhenCoverageLow(t *testing.T) {
	files := []PRFile{{Path: "src/a.ts", Additions: 10, Deletions: 0}}
	got := EstimateReviewTime("alice", "feat: huge refactor", 1000, 200, 50, files, true, defaultEstimateTimeBuckets())
	if got != "~60m+" {
		t.Errorf("EstimateReviewTime() = %q; want %q", got, "~60m+")
	}
}
