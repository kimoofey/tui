package ui

import (
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestAppleTerminalColorLimit(t *testing.T) {
	for _, tc := range []struct {
		program string
		profile colorprofile.Profile
		want    bool
	}{
		{"Apple_Terminal", colorprofile.TrueColor, true},
		{"Apple_Terminal", colorprofile.ANSI256, false},
		{"Apple_Terminal", colorprofile.ANSI, false},
		{"Apple_Terminal", colorprofile.ASCII, false},
		{"Apple_Terminal", colorprofile.NoTTY, false},
		{"ghostty", colorprofile.TrueColor, false},
		{"", colorprofile.TrueColor, false},
	} {
		if got := appleTerminalNeedsColorLimit(tc.program, tc.profile); got != tc.want {
			t.Errorf("program %q profile %s: got %v, want %v", tc.program, tc.profile, got, tc.want)
		}
	}
}
