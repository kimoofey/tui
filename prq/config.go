package prq

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed config.example.yaml
var embeddedConfig []byte

// Config holds all runtime settings (defaults → config file → CLI flags).
type Config struct {
	WatchRepos          []string `yaml:"watch_repos"`
	DaysAgo             int      `yaml:"days_ago"`
	SkipAlreadyReviewed bool     `yaml:"skip_already_reviewed"`
	SkipApproved        bool     `yaml:"skip_approved"`
	SkipBots            bool     `yaml:"skip_bots"`
	EstimateTimeBuckets []int    `yaml:"estimate_time_buckets"`
	OpencodeTerminal    string   `yaml:"opencode_terminal"`
	PageSize            int      `yaml:"page_size"`
	MaxReviewers        int      `yaml:"max_reviewers"`
	MaxPages            int      `yaml:"max_pages"`
}

func defaultConfig() Config {
	return Config{
		WatchRepos:          []string{},
		DaysAgo:             30,
		SkipAlreadyReviewed: true,
		SkipApproved:        true,
		SkipBots:            true,
		EstimateTimeBuckets: defaultEstimateTimeBuckets(),
		OpencodeTerminal:    "",
		PageSize:            100,
		MaxReviewers:        20,
		MaxPages:            3,
	}
}

func defaultEstimateTimeBuckets() []int {
	return []int{1, 2, 3, 5, 8, 12, 20, 30, 45, 60}
}

func (c Config) Validate() error {
	var errs []string
	if c.DaysAgo <= 0 || c.DaysAgo > 365 {
		errs = append(errs, fmt.Sprintf("days_ago must be 1–365, got %d", c.DaysAgo))
	}
	if c.PageSize <= 0 || c.PageSize > 100 {
		errs = append(errs, fmt.Sprintf("page_size must be 1–100, got %d", c.PageSize))
	}
	if c.MaxPages <= 0 || c.MaxPages > 10 {
		errs = append(errs, fmt.Sprintf("max_pages must be 1–10, got %d", c.MaxPages))
	}
	if c.MaxReviewers <= 0 || c.MaxReviewers > 100 {
		errs = append(errs, fmt.Sprintf("max_reviewers must be 1–100, got %d", c.MaxReviewers))
	}
	if len(errs) > 0 {
		return errors.New("invalid config: " + errs[0])
	}
	return nil
}

// GlobalConfigPath returns ~/.config/prq/config.yaml, respecting
// XDG_CONFIG_HOME if set.
func GlobalConfigPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = os.Getenv("HOME")
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "prq", "config.yaml")
}

// LoadConfig builds a Config by layering: defaults → global file.
// A missing file is silently skipped; other read and YAML errors are returned.
func LoadConfig() (Config, error) {
	cfg := defaultConfig()
	if err := mergeYAML(&cfg, GlobalConfigPath()); err != nil {
		return cfg, fmt.Errorf("config %s: %w", GlobalConfigPath(), err)
	}
	return cfg, nil
}

func mergeYAML(cfg *Config, path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && err != io.EOF {
		return err
	}
	return nil
}

// ScaffoldConfig writes the embedded config.example.yaml to the global config
// path. If the destination already exists, it is left unchanged.
func ScaffoldConfig() error {
	dest := GlobalConfigPath()
	if _, err := os.Stat(dest); err == nil {
		fmt.Printf("Config already exists at %s — edit it to update your settings.\n", dest)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dest, embeddedConfig, 0o644); err != nil {
		return err
	}
	fmt.Printf("Config created at %s\n", dest)
	fmt.Println("Open it and update watch_repos, then run prq again.")
	return nil
}
