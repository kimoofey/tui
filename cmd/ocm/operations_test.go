//go:build !mock

package main

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
	db "github.com/kimoofey/tui/ocm/sources/opencode"
)

func testExecutable(t *testing.T, script string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "bin with 'quotes' $and `ticks`")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "opencode")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func readNULFields(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
}

// Invoked only in a subprocess by the fake OpenCode executable below. It
// simulates cascading deletion against the disposable test database.
func TestDeleteProcessHelper(t *testing.T) {
	if os.Getenv("OCM_DELETE_HELPER") != "1" {
		return
	}
	_, args, ok := strings.Cut(strings.Join(os.Args, "\x00"), "--\x00")
	parts := strings.Split(args, "\x00")
	if !ok || len(parts) < 3 || parts[0] != "session" || parts[1] != "delete" {
		t.Fatalf("unexpected helper args: %q", parts)
	}
	if parts[2] == "fail" {
		t.Fatal("simulated delete failure")
	}
	uri := (&url.URL{Scheme: "file", Path: os.Getenv("OPENCODE_DB"), RawQuery: "mode=rw"}).String()
	writer, err := sql.Open("sqlite", uri)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	if _, err := writer.Exec(`PRAGMA foreign_keys = ON;
		DELETE FROM session_v2 WHERE id = ? OR parent_id = ?`, parts[2], parts[2]); err != nil {
		t.Fatal(err)
	}
}

func deleteTestStore(t *testing.T) *db.Store {
	t.Helper()
	dataDir := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dataDir, "opencode.db")
	writer, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	_, err = writer.Exec(`
		CREATE TABLE session_v2 (id TEXT PRIMARY KEY, parent_id TEXT, fork_session_id TEXT, title TEXT,
			directory TEXT, time_created INTEGER, time_updated INTEGER, cost REAL);
		CREATE TABLE session_message (session_id TEXT REFERENCES session_v2(id) ON DELETE CASCADE,
			type TEXT, time_created INTEGER, data TEXT);
		CREATE TABLE event (aggregate_id TEXT, type TEXT, created INTEGER, data TEXT);
		INSERT INTO session_v2 VALUES
			('root', NULL, NULL, 'Root', '/p', 1, 3, 1),
			('child', 'root', NULL, 'Child', '/p', 1, 2, 1),
			('keep', NULL, NULL, 'Keep', '/q', 1, 1, 1);
		INSERT INTO session_message VALUES ('root', 'assistant', ?, '{"cost":1}'),
			('child', 'assistant', ?, '{"cost":2}'), ('keep', 'assistant', ?, '{"cost":3}');`,
		time.Now().UnixMilli()-1000, time.Now().UnixMilli()-1000, time.Now().UnixMilli()-1000)
	if err != nil {
		t.Fatal(err)
	}
	executable := testExecutable(t, `
if test "$1" = debug; then
 printf 'data %s\ndb %s\n' "$OCM_TEST_DATA" "$OCM_TEST_DB"
 exit 0
fi
exec "$OCM_TEST_BINARY" -test.run='^TestDeleteProcessHelper$' -- "$@"`)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCM_TEST_BINARY", binary)
	t.Setenv("OCM_DELETE_HELPER", "1")
	t.Setenv("PATH", filepath.Dir(executable))
	t.Setenv("OCM_TEST_DATA", dataDir)
	t.Setenv("OCM_TEST_DB", path)
	store, err := db.Discover()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestDeleteSession_RefreshesChildrenCountsAndCosts(t *testing.T) {
	store := deleteTestStore(t)
	app := service.New(store, nil, nil)
	ref := func(id string) session.Ref { return session.Ref{Source: store.Info().ID, ID: id} }
	result := app.Delete(context.Background(), []session.Ref{ref("root")}, service.LoadOptions{CostPeriod: "year"})
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	snap := result.Snapshot
	if snap.TotalCount != 1 || len(snap.Sessions) != 1 || snap.Sessions[0].ID != "keep" || snap.PeriodCost.Amount != 3 {
		t.Fatalf("reload after delete: %+v", snap)
	}
	last := app.Delete(context.Background(), []session.Ref{ref("keep")}, service.LoadOptions{})
	if last.Err != nil || last.Snapshot.TotalCount != 0 || len(last.Snapshot.Sessions) != 0 {
		t.Fatalf("last deletion: %+v", last)
	}
}
func TestBulkDelete_RefreshesOnPartialFailure(t *testing.T) {
	store := deleteTestStore(t)
	app := service.New(store, nil, nil)
	result := app.Delete(context.Background(), []session.Ref{{Source: store.Info().ID, ID: "root"}, {Source: store.Info().ID, ID: "fail"}}, service.LoadOptions{RootOnly: true})
	if result.Err == nil || result.Count != 1 || result.Snapshot == nil || result.Snapshot.TotalCount != 1 || len(result.Snapshot.Sessions) != 1 {
		t.Fatalf("bulk result: %+v", result)
	}
}
func TestModel_CustomStorageCannotPrune(t *testing.T) {
	_ = deleteTestStore(t)
	t.Setenv("OCM_TEST_DATA", filepath.Join(t.TempDir(), "opencode"))
	custom, err := db.Discover()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = custom.Close() }()
	app := service.New(custom, nil, nil)
	model := ocm.New(ocm.Options{Service: app, Capabilities: app.Capabilities(), OrphanCount: 42}, 100, 30)
	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'p'})
	if cmd != nil || !strings.Contains(updated.View().Content, "pruning unavailable") {
		t.Fatalf("custom prune: %v", updated.View())
	}
}
