//go:build !mock

package main

import (
	"database/sql"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIProcess(t *testing.T) {
	if os.Getenv("OCM_CLI_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"ocm"}, os.Args[i+1:]...)
			break
		}
	}
	flag.CommandLine = flag.NewFlagSet("ocm", flag.ExitOnError)
	os.Exit(run())
}
func TestCLIFlagsAndFailures(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	dir := t.TempDir()
	data := filepath.Join(dir, "opencode")
	if err := os.Mkdir(data, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(data, "opencode.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE session (id TEXT,title TEXT,directory TEXT,time_created INTEGER,time_updated INTEGER,cost REAL,parent_id TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(bin, "opencode")
	marker := filepath.Join(dir, "discovered")
	script := `#!/bin/sh
printf 'called' > "$OCM_CLI_MARKER"
if test "$OCM_CLI_FAIL" = 1; then exit 7; fi
printf 'data %s\ndb %s\n' "$OCM_CLI_DATA" "$OCM_CLI_DB"
`
	if err := os.WriteFile(executable, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name     string
		args     []string
		fail     bool
		db       string
		code     int
		want     string
		discover bool
	}{
		{name: "version", args: []string{"--version"}, want: "ocm ", code: 0},
		{name: "help", args: []string{"--help"}, want: "-cost-period", code: 0},
		{name: "invalid period", args: []string{"--cost-period", "fortnight"}, want: "invalid cost period", code: 2},
		{name: "empty", want: "No OpenCode sessions found.", code: 0, discover: true},
		{name: "discovery failure", fail: true, want: "debug paths failed", code: 1, discover: true},
		{name: "load failure", db: filepath.Join(data, "missing.db"), want: "access database", code: 1, discover: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Remove(marker)
			args := append([]string{"-test.run=^TestCLIProcess$", "--"}, tt.args...)
			cmd := exec.Command(binary, args...)
			dbPath := tt.db
			if dbPath == "" {
				dbPath = path
			}
			fail := "0"
			if tt.fail {
				fail = "1"
			}
			cmd.Env = append(os.Environ(), "OCM_CLI_HELPER=1", "PATH="+bin, "OCM_CLI_DATA="+data, "OCM_CLI_DB="+dbPath, "OCM_CLI_MARKER="+marker, "OCM_CLI_FAIL="+fail)
			output, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				exit, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			if code != tt.code || !strings.Contains(string(output), tt.want) {
				t.Fatalf("code=%d output=%q err=%v", code, output, err)
			}
			_, err = os.Stat(marker)
			if (err == nil) != tt.discover {
				t.Fatalf("unexpected discovery: %v", err)
			}
			if tt.name == "help" {
				for _, flag := range []string{"-sessions", "-cost-period", "-version"} {
					if !strings.Contains(string(output), flag) {
						t.Fatalf("missing flag %s", flag)
					}
				}
			}
		})
	}
}
