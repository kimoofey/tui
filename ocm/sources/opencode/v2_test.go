package opencode

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Minimal V2 schema from OpenCode v2.1.4. No V1 tables are required.
const v2Schema = `
CREATE TABLE session_v2 (
	id TEXT PRIMARY KEY, title TEXT, directory TEXT NOT NULL,
	time_created INTEGER NOT NULL, time_updated INTEGER NOT NULL,
	cost REAL NOT NULL DEFAULT 0, parent_id TEXT, fork_session_id TEXT
);
CREATE TABLE session_message (
	id TEXT PRIMARY KEY, session_id TEXT NOT NULL, type TEXT NOT NULL,
	seq INTEGER NOT NULL, time_created INTEGER NOT NULL, time_updated INTEGER NOT NULL,
	data TEXT NOT NULL
);
CREATE TABLE event (
	id TEXT PRIMARY KEY, aggregate_id TEXT NOT NULL, seq INTEGER NOT NULL,
	type TEXT NOT NULL, data TEXT NOT NULL, created INTEGER NOT NULL
);`

func execTestSQL(t *testing.T, conn *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func testWriter(t *testing.T, path string) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", sqliteURI(path, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func createV2Store(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode.db")
	writer := testWriter(t, path)
	execTestSQL(t, writer, v2Schema)
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, writer
}

func TestLoadSessions_V2(t *testing.T) {
	store, writer := createV2Store(t)
	execTestSQL(t, writer, `INSERT INTO session_v2 VALUES
		('root', 'V2 root', '/project', 1000, 2000, 1.25, NULL, NULL),
		('child', NULL, '/project/sub', 1100, 3000, 0.5, 'root', NULL),
		('fork', 'Fork', '/other', 1200, 2500, 0.75, NULL, 'root')`)
	for _, rootOnly := range []bool{false, true} {
		sessions, err := store.loadSessions(context.Background(), rootOnly)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, session := range sessions {
			ids = append(ids, session.ID)
		}
		want := []string{"child", "fork", "root"}
		if rootOnly {
			want = []string{"fork", "root"}
		} else if sessions[0].Title != "Untitled session" {
			t.Fatalf("null title = %q", sessions[0].Title)
		}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("rootOnly=%v: ids=%v, want %v", rootOnly, ids, want)
		}
		root := sessions[len(sessions)-1]
		if root.Cost != 1.25 || root.Created.UnixMilli() != 1000 || root.Updated.UnixMilli() != 2000 || root.Directory != "/project" {
			t.Fatalf("root = %+v", root)
		}
	}
	if count, err := store.sessionCount(context.Background()); err != nil || count != 3 {
		t.Fatalf("count=%d, error=%v", count, err)
	}
}

func TestSchemaMigration_V2Authoritative(t *testing.T) {
	store := createTestStore(t, testSessions)
	writer := testWriter(t, store.Path())
	if sessions, err := store.loadSessions(context.Background(), false); err != nil || len(sessions) != 3 {
		t.Fatalf("before migration: %v, %v", sessions, err)
	}
	execTestSQL(t, writer, v2Schema)
	if sessions, err := store.loadSessions(context.Background(), false); err != nil || len(sessions) != 0 {
		t.Fatalf("empty V2 must override populated V1: %v, %v", sessions, err)
	}
	if count, err := store.sessionCount(context.Background()); err != nil || count != 0 {
		t.Fatalf("empty V2 count=%d, error=%v", count, err)
	}
	execTestSQL(t, writer, `INSERT INTO session_v2 VALUES
		('sess-1', 'Updated V2 title', '/new', 1000, 4000, 9, NULL, NULL),
		('new-v2', 'New', '/new', 2000, 5000, 1, NULL, NULL)`)
	sessions, err := store.loadSessions(context.Background(), false)
	if err != nil || len(sessions) != 2 {
		t.Fatalf("mixed sessions: %v, %v", sessions, err)
	}
	if sessions[1].Title != "Updated V2 title" || sessions[1].Cost != 9 {
		t.Fatalf("used frozen V1 fields: %+v", sessions[1])
	}
	// Deleting a V2 row must not resurrect its legacy copy on reload.
	execTestSQL(t, writer, `DELETE FROM session_v2 WHERE id = 'sess-1'`)
	if count, err := store.sessionCount(context.Background()); err != nil || count != 1 {
		t.Fatalf("count after delete=%d, error=%v", count, err)
	}
}

func TestLoadSessions_V1RootFilter(t *testing.T) {
	store := createTestStore(t, testSessions)
	writer := testWriter(t, store.Path())
	execTestSQL(t, writer, `UPDATE session SET parent_id = 'sess-1' WHERE id = 'sess-2'`)
	sessions, err := store.loadSessions(context.Background(), true)
	if err != nil || len(sessions) != 2 {
		t.Fatalf("roots: %v, %v", sessions, err)
	}
	if count, err := store.sessionCount(context.Background()); err != nil || count != 3 {
		t.Fatalf("unfiltered count=%d, error=%v", count, err)
	}
}

func TestOpen_RejectsMissingAndUnsupportedStorage(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	if _, err := Open(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing error=%v", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("opening created a missing database")
	}
	for _, schema := range []string{
		`CREATE TABLE unrelated (id TEXT)`,
		`CREATE TABLE session (id TEXT)`,
		`CREATE TABLE session_v2 (id TEXT)`,
	} {
		path := filepath.Join(t.TempDir(), "opencode.db")
		writer := testWriter(t, path)
		execTestSQL(t, writer, schema)
		if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "unsupported OpenCode schema") {
			t.Fatalf("schema %q: error=%v", schema, err)
		}
	}
	store := createTestStore(t, testSessions)
	execTestSQL(t, testWriter(t, store.Path()), `CREATE TABLE session_v2 (id TEXT)`)
	if _, err := store.loadSessions(context.Background(), false); err == nil {
		t.Fatal("incompatible V2 silently fell back to V1")
	}
	if _, err := Open(":memory:"); err == nil {
		t.Fatal("accepted in-memory storage")
	}
}

func TestPeriodCost_V2(t *testing.T) {
	store, writer := createV2Store(t)
	execTestSQL(t, writer, `INSERT INTO session_v2 VALUES
		('root', 'Root', '/p', 500, 3000, 999, NULL, NULL),
		('child', 'Child', '/p', 500, 3000, 999, 'root', NULL),
		('fork', 'Fork', '/p', 2000, 3000, 999, NULL, 'root')`)
	execTestSQL(t, writer, `INSERT INTO session_message VALUES
		('start', 'root', 'assistant', 1, 1000, 1000, '{"cost":1}'),
		('child', 'child', 'assistant', 1, 1500, 1500, '{"cost":2}'),
		('copied', 'fork', 'assistant', 1, 1500, 1500, '{"cost":99}'),
		('new', 'fork', 'assistant', 2, 2000, 2000, '{"cost":3}'),
		('end', 'root', 'assistant', 2, 3000, 3000, '{"cost":99}'),
		('before', 'root', 'assistant', 3, 999, 999, '{"cost":99}'),
		('missing-cost', 'root', 'assistant', 4, 1500, 1500, '{}'),
		('null-cost', 'root', 'assistant', 5, 1500, 1500, '{"cost":null}'),
		('user', 'root', 'user', 6, 1500, 1500, '{"cost":99}'),
		('compaction', 'root', 'compaction', 7, 1500, 1500, '{"cost":0.5}'),
		('deleted', 'absent', 'assistant', 1, 1500, 1500, '{"cost":99}')`)
	execTestSQL(t, writer, `INSERT INTO event VALUES
		('e1', 'root', 1, 'session.usage.recorded.1', '{"source":"compaction","cost":0.5}', 1500),
		('e2', 'root', 2, 'session.usage.recorded.1', '{"source":"title","cost":99}', 1500),
		('e3', 'root', 3, 'session.compaction.ended.1', '{"cost":0.5}', 1500),
		('e4', 'absent', 1, 'session.usage.recorded.1', '{"source":"compaction","cost":99}', 1500),
		('e5', 'root', 4, 'session.usage.recorded.1', '{"source":"compaction","cost":99}', 3000)`)
	// Mixed databases must never sum frozen V1 message copies as well.
	execTestSQL(t, writer, `CREATE TABLE message (time_created INTEGER, data TEXT);
		INSERT INTO message VALUES (1500, '{"cost":99}')`)
	for _, tt := range []struct {
		start, end int64
		want       float64
	}{{1000, 3000, 6.5}, {2000, 3000, 3}, {4000, 5000, 0}, {1000, 1000, 0}} {
		cost, err := store.periodCost(context.Background(), tt.start, tt.end)
		if err != nil || math.Abs(cost-tt.want) > 1e-9 {
			t.Fatalf("cost[%d,%d)=%v, want=%v, error=%v", tt.start, tt.end, cost, tt.want, err)
		}
	}
}

func TestPruneOrphans_V2(t *testing.T) {
	store, writer := createV2Store(t)
	execTestSQL(t, writer, `INSERT INTO session_v2 VALUES ('v2-only', NULL, '/p', 1, 1, 0, NULL, NULL)`)
	if err := os.MkdirAll(store.diffDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"v2-only.json", "orphan.json", "ignore.txt"} {
		if err := os.WriteFile(filepath.Join(store.diffDir(), name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if count, size, err := store.orphanStats(context.Background()); err != nil || count != 1 || size != 2 {
		t.Fatalf("stats=%d/%d, error=%v", count, size, err)
	}
	if count, size, err := store.pruneOrphans(context.Background()); err != nil || count != 1 || size != 2 {
		t.Fatalf("pruned=%d/%d, error=%v", count, size, err)
	}
	for _, name := range []string{"v2-only.json", "ignore.txt"} {
		if _, err := os.Stat(filepath.Join(store.diffDir(), name)); err != nil {
			t.Fatalf("removed live/ignored file: %v", err)
		}
	}
}

func TestCustomStorage_PruneDisabledAndVacuumBoundToPath(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(filepath.Join(dataDir, "storage", "session_diff"), 0o755); err != nil {
		t.Fatal(err)
	}
	sharedDiff := filepath.Join(dataDir, "storage", "session_diff", "other-db-session.json")
	if err := os.WriteFile(sharedDiff, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Exercise SQLite URI escaping as well as independence from the data dir.
	path := filepath.Join(t.TempDir(), "custom database ?#.db")
	writer := testWriter(t, path)
	execTestSQL(t, writer, v2Schema)
	store, err := openStorage(storageContext{DBPath: path, DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if count, size, err := store.orphanStats(context.Background()); err != nil || count != 0 || size != 0 {
		t.Fatalf("custom stats=%d/%d, error=%v", count, size, err)
	}
	if _, _, err := store.pruneOrphans(context.Background()); err == nil {
		t.Fatal("custom database allowed pruning")
	}
	if _, err := os.Stat(sharedDiff); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.vacuumDB(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "opencode.db")); !os.IsNotExist(err) {
		t.Fatal("maintenance created a different database")
	}
	if _, err := store.conn.Exec(`DELETE FROM session_v2`); err == nil {
		t.Fatal("store connection is writable")
	}
}
