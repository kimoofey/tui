package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// createTestDB creates a temporary SQLite database with the session table
// and returns its path. The caller should defer os.Remove on the path.
func createTestDB(t *testing.T, sessions []Session) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "opencode.db")

	conn, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer conn.Close()

	_, err = conn.Exec(`CREATE TABLE session (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		directory TEXT NOT NULL,
		time_created INTEGER NOT NULL,
		time_updated INTEGER NOT NULL,
		cost REAL NOT NULL DEFAULT 0,
		parent_id TEXT
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	for _, s := range sessions {
		_, err = conn.Exec(
			`INSERT INTO session (id, title, directory, time_created, time_updated, cost, parent_id) VALUES (?,?,?,?,?,?,?)`,
			s.ID, s.Title, s.Directory,
			s.Created.UnixMilli(), s.Updated.UnixMilli(),
			s.Cost, nil,
		)
		if err != nil {
			t.Fatalf("insert session %s: %v", s.ID, err)
		}
	}

	return dbPath
}

var testSessions = []Session{
	{
		ID:        "sess-1",
		Title:     "First session",
		Directory: "/home/user/projects/alpha",
		Created:   time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
		Updated:   time.Date(2025, 5, 1, 12, 0, 0, 0, time.UTC),
		Cost:      1.23,
	},
	{
		ID:        "sess-2",
		Title:     "Second session",
		Directory: "/home/user/projects/beta",
		Created:   time.Date(2025, 2, 1, 10, 0, 0, 0, time.UTC),
		Updated:   time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC),
		Cost:      0.5,
	},
	{
		ID:        "sess-3",
		Title:     "Third session",
		Directory: "/home/user/projects/gamma",
		Created:   time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC),
		Updated:   time.Date(2025, 4, 1, 8, 0, 0, 0, time.UTC),
		Cost:      0,
	},
}

func TestLoadSessions_ReturnsAllSortedByUpdatedDesc(t *testing.T) {
	dbPath := createTestDB(t, testSessions)

	sessions, err := LoadSessions(dbPath, false)
	if err != nil {
		t.Fatalf("LoadSessions: %v", err)
	}

	if len(sessions) != 3 {
		t.Fatalf("got %d sessions; want 3", len(sessions))
	}

	// should be sorted by time_updated DESC: sess-2, sess-1, sess-3
	wantOrder := []string{"sess-2", "sess-1", "sess-3"}
	for i, want := range wantOrder {
		if sessions[i].ID != want {
			t.Errorf("sessions[%d].ID = %q; want %q", i, sessions[i].ID, want)
		}
	}
}

func TestLoadSessions_Fields(t *testing.T) {
	dbPath := createTestDB(t, testSessions[:1])

	sessions, err := LoadSessions(dbPath, false)
	if err != nil {
		t.Fatalf("LoadSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions; want 1", len(sessions))
	}

	s := sessions[0]
	if s.Title != "First session" {
		t.Errorf("Title = %q; want %q", s.Title, "First session")
	}
	if s.Directory != "/home/user/projects/alpha" {
		t.Errorf("Directory = %q; want %q", s.Directory, "/home/user/projects/alpha")
	}
	if s.Cost != 1.23 {
		t.Errorf("Cost = %v; want 1.23", s.Cost)
	}
	if s.Created.IsZero() {
		t.Error("Created is zero")
	}
}

func TestLoadSessions_Empty(t *testing.T) {
	dbPath := createTestDB(t, nil)

	sessions, err := LoadSessions(dbPath, false)
	if err != nil {
		t.Fatalf("LoadSessions on empty db: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("got %d sessions; want 0", len(sessions))
	}
}

func TestSessionCount(t *testing.T) {
	dbPath := createTestDB(t, testSessions)

	count, err := SessionCount(dbPath)
	if err != nil {
		t.Fatalf("SessionCount: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d; want 3", count)
	}
}

func TestSessionCount_Empty(t *testing.T) {
	dbPath := createTestDB(t, nil)

	count, err := SessionCount(dbPath)
	if err != nil {
		t.Fatalf("SessionCount on empty db: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d; want 0", count)
	}
}

func TestDBStats(t *testing.T) {
	dbPath := createTestDB(t, testSessions)

	used, total, err := DBStats(dbPath)
	if err != nil {
		t.Fatalf("DBStats: %v", err)
	}
	if total <= 0 {
		t.Errorf("total = %d; want > 0", total)
	}
	if used <= 0 {
		t.Errorf("used = %d; want > 0", used)
	}
	if used > total {
		t.Errorf("used (%d) > total (%d)", used, total)
	}
}

func TestOrphanStats_NoOrphans(t *testing.T) {
	dbPath := createTestDB(t, testSessions)

	// Create the session_diff dir but no files
	diffDir := filepath.Join(filepath.Dir(dbPath), "storage", "session_diff")
	if err := os.MkdirAll(diffDir, 0o755); err != nil {
		t.Fatal(err)
	}

	count, bytes, err := OrphanStats(dbPath)
	if err != nil {
		t.Fatalf("OrphanStats: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d; want 0", count)
	}
	if bytes != 0 {
		t.Errorf("bytes = %d; want 0", bytes)
	}
}

func TestOrphanStats_WithOrphans(t *testing.T) {
	dbPath := createTestDB(t, testSessions[:1])

	diffDir := filepath.Join(filepath.Dir(dbPath), "storage", "session_diff")
	if err := os.MkdirAll(diffDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a diff file for a session NOT in the DB — orphan
	orphanFile := filepath.Join(diffDir, "orphan-id.json")
	if err := os.WriteFile(orphanFile, []byte(`{"data":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a diff file for sess-1 — not an orphan
	knownFile := filepath.Join(diffDir, "sess-1.json")
	if err := os.WriteFile(knownFile, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	count, bytes, err := OrphanStats(dbPath)
	if err != nil {
		t.Fatalf("OrphanStats: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d; want 1", count)
	}
	if bytes == 0 {
		t.Error("bytes = 0; want > 0")
	}
}
