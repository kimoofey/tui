package ocm

import (
	"testing"
	"time"
)

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost float64
		want string
	}{
		{0, "-"},
		{0.01, "$0.01"},
		{1.5, "$1.50"},
		{999.99, "$999.99"},
	}
	for _, tt := range tests {
		got := formatCost(tt.cost)
		if got != tt.want {
			t.Errorf("formatCost(%v) = %q; want %q", tt.cost, got, tt.want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1 << 10, "1 KB"},
		{1536, "2 KB"},
		{1 << 20, "1 MB"},
		{5 * (1 << 20), "5 MB"},
		{1 << 30, "1.0 GB"},
		{int64(2.5 * float64(1<<30)), "2.5 GB"},
	}
	for _, tt := range tests {
		got := formatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q; want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestShortenHome(t *testing.T) {
	tests := []struct {
		path string
		home string
		want string
	}{
		{"/home/user/projects/foo", "/home/user", "~/projects/foo"},
		{"/home/user", "/home/user", "~"},
		{"/etc/config", "/home/user", "/etc/config"},
		{"/home/user/projects/foo", "", "/home/user/projects/foo"},
		{"/other/path", "/home/user", "/other/path"},
	}
	for _, tt := range tests {
		got := shortenHome(tt.path, tt.home)
		if got != tt.want {
			t.Errorf("shortenHome(%q, %q) = %q; want %q", tt.path, tt.home, got, tt.want)
		}
	}
}

func TestFormatDateRelativeTo(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"30 seconds ago", now.Add(-30 * time.Second), "0m ago"},
		{"5 minutes ago", now.Add(-5 * time.Minute), "5m ago"},
		{"59 minutes ago", now.Add(-59 * time.Minute), "59m ago"},
		{"1 hour ago", now.Add(-1 * time.Hour), "1h ago"},
		{"3 hours ago", now.Add(-3 * time.Hour), "3h ago"},
		{"23 hours ago", now.Add(-23 * time.Hour), "23h ago"},
		{"1 day ago", now.Add(-24 * time.Hour), "1d ago"},
		{"3 days ago", now.Add(-3 * 24 * time.Hour), "3d ago"},
		{"6 days ago", now.Add(-6 * 24 * time.Hour), "6d ago"},
		{"7 days ago — absolute", now.Add(-7 * 24 * time.Hour), "May 25 2025"},
		{"older — absolute", now.Add(-30 * 24 * time.Hour), "May 02 2025"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDateRelativeTo(tt.t, now)
			if got != tt.want {
				t.Errorf("formatDateRelativeTo(%v) = %q; want %q", tt.t, got, tt.want)
			}
		})
	}
}
