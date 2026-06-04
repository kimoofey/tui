package ocm

import (
	"fmt"
	"strings"
	"time"
)

type CostPeriod string

const (
	CostPeriodWeek  CostPeriod = "week"
	CostPeriodMonth CostPeriod = "month"
	CostPeriodYear  CostPeriod = "year"
)

func ParseCostPeriod(s string) (CostPeriod, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch CostPeriod(s) {
	case CostPeriodWeek, CostPeriodMonth, CostPeriodYear:
		return CostPeriod(s), nil
	default:
		return "", fmt.Errorf("invalid cost period %q (expected: week|month|year)", s)
	}
}

func CostPeriodBounds(period CostPeriod, now time.Time) (time.Time, time.Time, error) {
	loc := now.Location()
	n := now.In(loc)

	switch period {
	case CostPeriodWeek:
		// Monday-start week in local time.
		daysSinceMonday := (int(n.Weekday()) + 6) % 7
		start := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -daysSinceMonday)
		return start, n, nil
	case CostPeriodMonth:
		start := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, loc)
		return start, n, nil
	case CostPeriodYear:
		start := time.Date(n.Year(), time.January, 1, 0, 0, 0, 0, loc)
		return start, n, nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("invalid cost period %q", period)
	}
}
