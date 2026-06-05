package prq

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

func EstimateReviewTime(author, title string, additions, deletions, changedFiles int, files []PRFile, filesTruncated bool, buckets []int) string {
	// Bot/dependency short-circuit
	if author == "dependabot[bot]" || author == "renovate[bot]" {
		return "~1m"
	}

	rawEstimate := estimateFromWeightedFiles(additions, deletions, changedFiles, files, filesTruncated)

	if strings.Contains(strings.ToLower(title), "bump") ||
		strings.Contains(strings.ToLower(title), "update") ||
		strings.Contains(strings.ToLower(title), "upgrade") ||
		strings.Contains(strings.ToLower(title), "chore(deps)") {
		rawEstimate *= 0.2
	}

	estimate := math.Max(1, math.Min(120, rawEstimate))

	return estimateToLabel(estimate, buckets)
}

func estimateFromWeightedFiles(additions, deletions, changedFiles int, files []PRFile, filesTruncated bool) float64 {
	if len(files) == 0 {
		return estimateFromAggregate(additions, deletions, changedFiles)
	}

	weightedLines := 0.0
	weightedFiles := 0.0
	for _, f := range files {
		w := fileTypeWeight(f.Path)
		weightedLines += (float64(f.Additions) + float64(f.Deletions)*deletionsWeight) * w
		weightedFiles += fileSwitchWeight(f.Path)
	}

	base := defaultBaseOverhead + (weightedLines / defaultReviewThroughput)
	contextPenalty := math.Max(0, weightedFiles-defaultContextFreeFiles) * defaultContextSwitchMins
	raw := base + contextPenalty

	fetchedCount := len(files)
	coverage := 1.0
	if changedFiles > 0 {
		coverage = float64(fetchedCount) / float64(changedFiles)
	}

	if filesTruncated || changedFiles > fetchedCount {
		if coverage < minCoverageForFileOnlyEst {
			return estimateFromAggregate(additions, deletions, changedFiles)
		}
		scaleUp := 1 / coverage
		if scaleUp > maxCoverageScaleUp {
			scaleUp = maxCoverageScaleUp
		}
		raw *= scaleUp
	}

	return raw
}

func estimateFromAggregate(additions, deletions, changedFiles int) float64 {
	effectiveLines := float64(additions) + float64(deletions)*deletionsWeight
	base := defaultBaseOverhead + (effectiveLines / defaultReviewThroughput)
	contextPenalty := math.Max(0, float64(changedFiles)-defaultContextFreeFiles) * defaultContextSwitchMins
	return base + contextPenalty
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
