package prq

import (
	_ "embed"
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
	MinApprovals        int      `yaml:"min_approvals"`
	SkipAlreadyReviewed bool     `yaml:"skip_already_reviewed"`
	SkipBots            bool     `yaml:"skip_bots"`
	OpencodeTerminal    string   `yaml:"opencode_terminal"`
	PageSize            int      `yaml:"page_size"`
	MaxReviewers        int      `yaml:"max_reviewers"`
	MaxPages            int      `yaml:"max_pages"`
}

func defaultConfig() Config {
	return Config{
		WatchRepos:          []string{},
		DaysAgo:             30,
		MinApprovals:        2,
		SkipAlreadyReviewed: true,
		SkipBots:            true,
		OpencodeTerminal:    "",
		PageSize:            100,
		MaxReviewers:        20,
		MaxPages:            3,
	}
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
// Missing file is silently skipped. Returns an error only on malformed YAML.
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
// path. Returns an error if the destination already exists.
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
