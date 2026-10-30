package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseStoragePaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(home, ".local", "share", "opencode")
	for _, tt := range []struct {
		name, output, override, xdg, wantDB string
		v2, standalone, prune               bool
	}{
		{"v1 default", "data       " + data, "", "", filepath.Join(data, "opencode.db"), false, false, true},
		{"v2 default", "data       " + data + "\ndb         " + filepath.Join(data, "opencode.db"), "", "", filepath.Join(data, "opencode.db"), true, false, true},
		{"v1 relative override", "data       " + data, "custom.db", "", filepath.Join(data, "custom.db"), false, false, false},
		{"v1 absolute override", "data       " + data, "/tmp/session data/custom.db", "", "/tmp/session data/custom.db", false, false, false},
		{"v2 authoritative path", "data       " + data + "\ndb         /tmp/other.db", "ignored.db", "", "/tmp/other.db", true, true, false},
		{"channel", "data       " + data + "\ndb         " + filepath.Join(data, "opencode-canary.db"), "", "", filepath.Join(data, "opencode-canary.db"), true, true, false},
		{"XDG and spaces", "home /unused\r\ndata\t/tmp/my data/opencode\r\ndb\t/tmp/my data/opencode/opencode.db\r\n", "", "/tmp/my data", "/tmp/my data/opencode/opencode.db", true, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENCODE_DB", tt.override)
			t.Setenv("XDG_DATA_HOME", tt.xdg)
			got, err := parseStoragePaths(tt.output, "/tmp/bin/opencode")
			if err != nil {
				t.Fatal(err)
			}
			if got.DBPath != tt.wantDB || got.V2CLI != tt.v2 || got.Standalone != tt.standalone || got.PruneSupported() != tt.prune {
				t.Fatalf("storage = %+v, prune=%v", got, got.PruneSupported())
			}
			if got.Executable != "/tmp/bin/opencode" {
				t.Fatalf("executable = %q", got.Executable)
			}
		})
	}
}

func TestParseStoragePaths_Invalid(t *testing.T) {
	t.Setenv("OPENCODE_DB", "")
	for _, output := range []string{
		"", "data relative/opencode", "data /tmp/opencode\ndb relative.db",
		"data /tmp/opencode\ndb :memory:", "data /tmp/opencode\ndb ",
		"data /tmp/opencode\ndata /tmp/other/opencode",
		"data /tmp/opencode\ndb /tmp/a.db\ndb /tmp/b.db",
	} {
		if _, err := parseStoragePaths(output, "opencode"); err == nil {
			t.Errorf("accepted invalid paths %q", output)
		}
	}
	t.Setenv("OPENCODE_DB", ":memory:")
	if _, err := parseStoragePaths("data /tmp/opencode", "opencode"); err == nil {
		t.Fatal("accepted legacy :memory: override")
	}
}

func fakeOpenCode(t *testing.T, script string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "bin with spaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "opencode")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return path
}

func TestDiscoverStorage(t *testing.T) {
	executable := fakeOpenCode(t, `
test "$#" = 2 && test "$1" = debug && test "$2" = paths || exit 2
printf 'data       %s\ndb         %s\n' "$OCM_TEST_DATA" "$OCM_TEST_DB"
`)
	t.Setenv("OCM_TEST_DATA", "/tmp/XDG data/opencode")
	t.Setenv("OCM_TEST_DB", "/tmp/databases/session ?#.db")
	storage, err := DiscoverStorage()
	if err != nil {
		t.Fatal(err)
	}
	if storage.DBPath != os.Getenv("OCM_TEST_DB") || storage.Executable != executable || !storage.V2CLI {
		t.Fatalf("storage = %+v", storage)
	}
}

func TestDiscoverStorage_Failures(t *testing.T) {
	t.Run("missing CLI", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, err := DiscoverStorage(); err == nil || !strings.Contains(err.Error(), "executable") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("CLI failed", func(t *testing.T) {
		fakeOpenCode(t, "exit 7\n")
		if _, err := DiscoverStorage(); err == nil || !strings.Contains(err.Error(), "debug paths failed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		fakeOpenCode(t, "exec /bin/sleep 10\n")
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if _, err := discoverStorage(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v", err)
		}
	})
}
