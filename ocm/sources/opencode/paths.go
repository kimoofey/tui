package opencode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// storageContext identifies one local OpenCode installation and its storage.
// V2CLI describes command capabilities, independently of the database schema.
type storageContext struct {
	DBPath     string
	DataDir    string
	Executable string
	V2CLI      bool
}

// discover asks OpenCode for paths without starting a server or opening
// its database. V1 reports the data directory but has no explicit db entry.
func discover() (storageContext, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return discoverStorage(ctx)
}

func discoverStorage(ctx context.Context) (storageContext, error) {
	executable, err := exec.LookPath("opencode")
	if err != nil {
		return storageContext{}, fmt.Errorf("find OpenCode executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return storageContext{}, fmt.Errorf("resolve OpenCode executable: %w", err)
	}
	cmd := exec.CommandContext(ctx, executable, "debug", "paths")
	cmd.WaitDelay = 250 * time.Millisecond
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return storageContext{}, fmt.Errorf("discover OpenCode paths: %w", ctx.Err())
	}
	if err != nil {
		return storageContext{}, fmt.Errorf("opencode debug paths failed (run it directly to diagnose): %w", err)
	}
	return parseStoragePaths(string(output), executable)
}

func parseStoragePaths(output, executable string) (storageContext, error) {
	paths := make(map[string]string)
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimLeft(strings.TrimSuffix(line, "\r"), " \t")
		i := strings.IndexAny(line, " \t")
		if i < 0 {
			if line != "data" && line != "db" {
				continue
			}
			i = len(line)
		}
		key, value := line[:i], line[i:]
		if key == "data" || key == "db" {
			if _, exists := paths[key]; exists {
				return storageContext{}, fmt.Errorf("duplicate %s in OpenCode paths", key)
			}
			paths[key] = strings.TrimLeft(value, " \t")
		}
	}
	data := paths["data"]
	if !filepath.IsAbs(data) || filepath.Base(filepath.Clean(data)) != "opencode" {
		return storageContext{}, fmt.Errorf("invalid OpenCode data directory %q", data)
	}
	filename, v2 := paths["db"]
	if !v2 {
		filename = os.Getenv("OPENCODE_DB")
		if filename == "" {
			filename = "opencode.db"
		}
		if filename != ":memory:" && !filepath.IsAbs(filename) {
			filename = filepath.Join(data, filename)
		}
	}
	if filename == ":memory:" {
		return storageContext{}, fmt.Errorf("OpenCode uses :memory: storage; OCM requires a database file")
	}
	if !filepath.IsAbs(filename) {
		return storageContext{}, fmt.Errorf("invalid OpenCode database path %q", filename)
	}
	storage := storageContext{
		DBPath: filepath.Clean(filename), DataDir: filepath.Clean(data),
		Executable: executable, V2CLI: v2,
	}
	return storage, nil
}

// PruneSupported is false when multiple databases may share legacy diff files.
func (s storageContext) PruneSupported() bool {
	return filepath.Clean(s.DBPath) == filepath.Join(s.DataDir, "opencode.db")
}
