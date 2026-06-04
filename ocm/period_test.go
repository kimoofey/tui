package ocm

import (
	"testing"
	"time"
)

func TestParseCostPeriod(t *testing.T) {
	tests := []struct {
		in      string
		want    CostPeriod
		wantErr bool
	}{
		{in: "week", want: CostPeriodWeek},
		{in: " month ", want: CostPeriodMonth},
		{in: "YEAR", want: CostPeriodYear},
		{in: "quarter", wantErr: true},
		{in: "", wantErr: true},
	}

	for _, tt := range tests {
		got, err := ParseCostPeriod(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseCostPeriod(%q): expected error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseCostPeriod(%q): unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseCostPeriod(%q) = %q; want %q", tt.in, got, tt.want)
		}
	}
}

func TestCostPeriodBounds_WeekStartsMonday(t *testing.T) {
	now := time.Date(2026, 6, 4, 15, 30, 0, 0, time.UTC)
	start, end, err := CostPeriodBounds(CostPeriodWeek, now)
	if err != nil {
		t.Fatalf("CostPeriodBounds: %v", err)
	}

	wantStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("week start = %v; want %v", start, wantStart)
	}
	if !end.Equal(now) {
		t.Errorf("week end = %v; want now %v", end, now)
	}
}
