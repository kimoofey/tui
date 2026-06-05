package prq

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

func EstimateReviewTime(author, title string, additions, deletions, changedFiles int, buckets []int) string {
	// Bot/dependency short-circuit
	if author == "dependabot[bot]" || author == "renovate[bot]" {
		return "~1m"
	}

	rawEstimate := 0.0

	// Effective lines
	effectiveLines := float64(additions) + (float64(deletions) * 0.4)

	// Base reading time
	baseMinutes := effectiveLines / 7.0

	// Context-switching penalty
	filePenalty := math.Max(0, float64(changedFiles)-3) * 1.5

	rawEstimate = baseMinutes + filePenalty

	if strings.Contains(strings.ToLower(title), "bump") ||
		strings.Contains(strings.ToLower(title), "update") ||
		strings.Contains(strings.ToLower(title), "upgrade") ||
		strings.Contains(strings.ToLower(title), "chore(deps)") {
		rawEstimate *= 0.2
	}

	estimate := math.Max(1, math.Min(120, rawEstimate))

	return estimateToLabel(estimate, buckets)
}

func estimateToLabel(estimate float64, buckets []int) string {
	validBuckets := normalizedBuckets(buckets)
	for _, b := range validBuckets {
		if estimate <= float64(b) {
			return "~" + strconv.Itoa(b) + "m"
		}
	}
	return "~" + strconv.Itoa(validBuckets[len(validBuckets)-1]) + "m+"
}

func normalizedBuckets(buckets []int) []int {
	if len(buckets) == 0 {
		return defaultEstimateTimeBuckets()
	}
	copyBuckets := make([]int, 0, len(buckets))
	for _, b := range buckets {
		if b > 0 {
			copyBuckets = append(copyBuckets, b)
		}
	}
	if len(copyBuckets) == 0 {
		return defaultEstimateTimeBuckets()
	}
	slices.Sort(copyBuckets)
	return slices.Compact(copyBuckets)
}
