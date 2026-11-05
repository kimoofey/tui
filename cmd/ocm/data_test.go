//go:build !mock

package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kimoofey/tui/internal/terminal"
	"github.com/kimoofey/tui/ocm/service"
	db "github.com/kimoofey/tui/ocm/sources/opencode"
)

func TestLoadData_V2CustomStorage(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data with spaces", "opencode")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dataDir, "custom.db")
	writer, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	_, err = writer.Exec(`
		CREATE TABLE session (id TEXT, title TEXT, directory TEXT, time_created INTEGER,
			time_updated INTEGER, cost REAL, parent_id TEXT);
		INSERT INTO session VALUES ('legacy-only', 'Old', '/p', 1, 1, 99, NULL);
		CREATE TABLE session_v2 (id TEXT PRIMARY KEY, title TEXT, directory TEXT, time_created INTEGER,
			time_updated INTEGER, cost REAL, parent_id TEXT, fork_session_id TEXT);
		INSERT INTO session_v2 VALUES ('root', NULL, '/p', 1, 2, 2, NULL, NULL),
			('child', 'Child', '/p', 1, 3, 3, 'root', NULL);
		CREATE TABLE session_message (session_id TEXT, type TEXT, time_created INTEGER, data TEXT);
		INSERT INTO session_message VALUES ('root', 'assistant', ?, '{"cost":2}'),
			('child', 'assistant', ?, '{"cost":3}');
		CREATE TABLE event (aggregate_id TEXT, type TEXT, created INTEGER, data TEXT);`,
		time.Now().UnixMilli()-1000, time.Now().UnixMilli()-1000)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	executable := filepath.Join(bin, "opencode")
	if err := os.WriteFile(executable, []byte(`#!/bin/sh
test "$#" = 2 && test "$1" = debug && test "$2" = paths || exit 2
printf 'data       %s\ndb         %s\n' "$OCM_TEST_DATA" "$OCM_TEST_DB"
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("OCM_TEST_DATA", dataDir)
	t.Setenv("OCM_TEST_DB", path)
	opts, err := loadData(true, service.CostPeriod("year"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opts.Close() }()
	if opts.TotalCount != 2 || len(opts.Sessions) != 1 || opts.Sessions[0].ID != "root" || opts.Sessions[0].Title != "Untitled session" {
		t.Fatalf("incorrect startup rows: count=%d, sessions=%v", opts.TotalCount, opts.Sessions)
	}
	if opts.PeriodCost != 5 || !opts.RootOnly || opts.DBTotal == 0 {
		t.Fatalf("incorrect startup stats: cost=%v, rootOnly=%v, bytes=%d", opts.PeriodCost, opts.RootOnly, opts.DBTotal)
	}
	// Capture the actual same-source launch spec without starting a terminal.
	capture := &captureLauncher{}
	source, err := db.Discover()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = source.Close() }()
	app := service.New(source, capture, nil)
	if err := app.Resume(context.Background(), opts.Sessions[0].Ref); err != nil {
		t.Fatal(err)
	}
	spec := capture.spec
	if spec.Executable != executable || spec.Env["OPENCODE_DB"] != path || spec.Env["XDG_DATA_HOME"] != filepath.Dir(dataDir) || strings.Join(spec.Args, " ") != "-s root --standalone" || opts.Capabilities.Orphans.Available {
		t.Fatalf("same-source routing: %+v", spec)
	}
}

type captureLauncher struct{ spec terminal.Command }

func (c *captureLauncher) Launch(spec terminal.Command) error { c.spec = spec; return nil }
