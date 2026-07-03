package prq

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	if cfg.DaysAgo != 30 {
		t.Errorf("DaysAgo = %d; want 30", cfg.DaysAgo)
	}
	if !cfg.SkipAlreadyReviewed {
		t.Error("SkipAlreadyReviewed = false; want true")
	}
	if !cfg.SkipApproved {
		t.Error("SkipApproved = false; want true")
	}
	if !cfg.SkipBots {
		t.Error("SkipBots = false; want true")
	}
	if cfg.PageSize != 100 {
		t.Errorf("PageSize = %d; want 100", cfg.PageSize)
	}
	if len(cfg.EstimateTimeBuckets) != 10 {
		t.Errorf("EstimateTimeBuckets len = %d; want 10", len(cfg.EstimateTimeBuckets))
	}
}

func TestMergeYAML_MissingFile(t *testing.T) {
	cfg := defaultConfig()
	err := mergeYAML(&cfg, "/nonexistent/path/config.yaml")
	if err != nil {
		t.Errorf("mergeYAML with missing file returned error: %v", err)
	}
	// defaults should be unchanged
	if cfg.DaysAgo != 30 {
		t.Errorf("DaysAgo changed after missing file: %d", cfg.DaysAgo)
	}
}

func TestMergeYAML_OverridesFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `days_ago: 14
skip_bots: false
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := defaultConfig()
	if err := mergeYAML(&cfg, path); err != nil {
		t.Fatalf("mergeYAML returned error: %v", err)
	}

	if cfg.DaysAgo != 14 {
		t.Errorf("DaysAgo = %d; want 14", cfg.DaysAgo)
	}
	if cfg.SkipBots {
		t.Error("SkipBots = true; want false")
	}
	if len(cfg.EstimateTimeBuckets) != 10 {
		t.Errorf("EstimateTimeBuckets len = %d; want 10 (default)", len(cfg.EstimateTimeBuckets))
	}
	// unset fields keep defaults
	if cfg.PageSize != 100 {
		t.Errorf("PageSize = %d; want 100 (default)", cfg.PageSize)
	}
}

func TestMergeYAML_EstimateTimeBuckets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `estimate_time_buckets:
  - 1
  - 2
  - 4
  - 8
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := defaultConfig()
	if err := mergeYAML(&cfg, path); err != nil {
		t.Fatalf("mergeYAML returned error: %v", err)
	}

	want := []int{1, 2, 4, 8}
	if len(cfg.EstimateTimeBuckets) != len(want) {
		t.Fatalf("EstimateTimeBuckets len = %d; want %d", len(cfg.EstimateTimeBuckets), len(want))
	}
	for i := range cfg.EstimateTimeBuckets {
		if cfg.EstimateTimeBuckets[i] != want[i] {
			t.Fatalf("EstimateTimeBuckets[%d] = %d; want %d", i, cfg.EstimateTimeBuckets[i], want[i])
		}
	}
}

func TestMergeYAML_UnknownFieldReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `days_ago: 7
unknown_field: oops
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := defaultConfig()
	err := mergeYAML(&cfg, path)
	if err == nil {
		t.Error("mergeYAML with unknown field should return error (KnownFields=true)")
	}
}

func TestMergeYAML_WatchRepos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `watch_repos:
  - owner/repo-a
  - owner/repo-b
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := defaultConfig()
	if err := mergeYAML(&cfg, path); err != nil {
		t.Fatalf("mergeYAML returned error: %v", err)
	}

	if len(cfg.WatchRepos) != 2 {
		t.Fatalf("WatchRepos len = %d; want 2", len(cfg.WatchRepos))
	}
	if cfg.WatchRepos[0] != "owner/repo-a" {
		t.Errorf("WatchRepos[0] = %q; want %q", cfg.WatchRepos[0], "owner/repo-a")
	}
}

func TestValidate_DefaultsPass(t *testing.T) {
	cfg := defaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
}

func TestValidate_InvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"days_ago zero", func(c *Config) { c.DaysAgo = 0 }},
		{"days_ago negative", func(c *Config) { c.DaysAgo = -1 }},
		{"days_ago too large", func(c *Config) { c.DaysAgo = 366 }},
		{"page_size zero", func(c *Config) { c.PageSize = 0 }},
		{"page_size too large", func(c *Config) { c.PageSize = 101 }},
		{"max_pages zero", func(c *Config) { c.MaxPages = 0 }},
		{"max_pages too large", func(c *Config) { c.MaxPages = 11 }},
		{"max_reviewers zero", func(c *Config) { c.MaxReviewers = 0 }},
		{"max_reviewers too large", func(c *Config) { c.MaxReviewers = 101 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}
