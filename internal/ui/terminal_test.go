package ui

import "testing"

func TestResolveTerminal(t *testing.T) {
	tests := []struct {
		name    string
		cfgVal  string
		termEnv string
		want    string
	}{
		{
			name:   "config override takes precedence",
			cfgVal: "kitty --",
			want:   "kitty --",
		},
		{
			name:    "ghostty detected",
			termEnv: "ghostty",
			want:    "ghostty",
		},
		{
			name:    "iterm2 detected",
			termEnv: "iTerm.app",
			want:    "iterm2",
		},
		{
			name:    "apple terminal detected",
			termEnv: "Apple_Terminal",
			want:    "terminal",
		},
		{
			name:    "unknown falls back to terminal",
			termEnv: "xterm-256color",
			want:    "terminal",
		},
		{
			name: "empty env falls back to terminal",
			want: "terminal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TERM_PROGRAM", tt.termEnv)
			got := ResolveTerminal(tt.cfgVal)
			if got != tt.want {
				t.Errorf("ResolveTerminal(%q) with TERM_PROGRAM=%q = %q; want %q",
					tt.cfgVal, tt.termEnv, got, tt.want)
			}
		})
	}
}
