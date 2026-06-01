package ui

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"empty string", "", 10, ""},
		{"fits exactly", "hello", 5, "hello"},
		{"fits under", "hi", 10, "hi"},
		{"truncated", "hello world", 8, "hello w…"},
		{"max 1 non-empty", "abc", 1, "…"},
		{"max 1 single char fits", "a", 1, "a"},
		{"max 0", "abc", 0, "…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q; want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}
