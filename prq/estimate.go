package prq

import (
	"math"
	"strings"
)

func EstimateReviewTime(author, title string, additions, deletions, changedFiles int) string {
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

	switch {
	case estimate <= 1:
		return "~1m"
	case estimate <= 2:
		return "~2m"
	case estimate <= 5:
		return "~5m"
	case estimate <= 10:
		return "~10m"
	case estimate <= 15:
		return "~15m"
	case estimate <= 20:
		return "~20m"
	case estimate <= 30:
		return "~30m"
	case estimate <= 45:
		return "~45m"
	default:
		return "~60m+"
	}
}
