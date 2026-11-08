package history

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	History struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"history"`
}

func ConfigPath() (string, error) { return xdgPath("XDG_CONFIG_HOME", ".config", "config.yaml") }
func ArchivePath() (string, error) {
	return xdgPath("XDG_DATA_HOME", ".local/share", "history.sqlite3")
}
func xdgPath(env, fallback, file string) (string, error) {
	base := os.Getenv(env)
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, fallback)
	}
	return filepath.Join(base, "ocm", file), nil
}

// LoadConfig accepts an explicit path so callers/tests need no access to user files.
func LoadConfig(path string) (Config, error) { return ReadConfig(path, os.ReadFile) }
func ReadConfig(path string, read func(string) ([]byte, error)) (Config, error) {
	var c Config
	b, err := read(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("history config %s: %w", path, err)
	}
	d := yaml.NewDecoder(bytes.NewReader(b))
	d.KnownFields(true)
	if err = d.Decode(&c); err != nil && err != io.EOF {
		return Config{}, fmt.Errorf("history config %s: %w", path, err)
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return Config{}, fmt.Errorf("history config %s: expected one YAML document", path)
	}
	return c, nil
}
