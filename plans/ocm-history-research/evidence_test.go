// Package research contains isolated evidence, never imported by production.
package research

import (
	"context"
	"database/sql"
	"net/url"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/kimoofey/tui/ocm/sources/opencode"
	_ "modernc.org/sqlite"
)

// Minimal legacy shape: no lineage evidence is present. Do not infer a
// supported history version from the browser accepting these columns.
const legacySchema = `
CREATE TABLE session (
 id TEXT PRIMARY KEY, title TEXT NOT NULL, directory TEXT NOT NULL,
 time_created INTEGER NOT NULL, time_updated INTEGER NOT NULL,
 cost REAL NOT NULL DEFAULT 0, parent_id TEXT
);
CREATE TABLE message (
 id TEXT PRIMARY KEY, session_id TEXT NOT NULL,
 time_created INTEGER NOT NULL, time_updated INTEGER NOT NULL, data TEXT NOT NULL
);
CREATE INDEX message_session_id_idx ON message(session_id);
`

func execute(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic.db")
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=rwc"}).String())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	execute(t, db, "PRAGMA journal_mode=WAL")
	execute(t, db, legacySchema)
	return path, db
}

func openReader(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}).String())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func period(t *testing.T, path string) float64 {
	t.Helper()
	source, err := opencode.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = source.Close() }()
	cost, err := source.PeriodCost(context.Background(), time.UnixMilli(0), time.UnixMilli(10000))
	if err != nil {
		t.Fatal(err)
	}
	return cost.Amount
}

func TestHistoryEnvironment(t *testing.T) {
	_, db := fixture(t)
	var version string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil {
		t.Fatal(err)
	}
	t.Logf("Go=%s SQLite=%s OS=%s arch=%s", runtime.Version(), version, runtime.GOOS, runtime.GOARCH)
}

func TestHistoryV1ForkEvidence(t *testing.T) {
	path, db := fixture(t)
	execute(t, db, `INSERT INTO session VALUES
 ('original','Original','/synthetic',1000,3000,3,NULL),
 ('copy','Copy','/synthetic',2000,3000,4,NULL);
 INSERT INTO message VALUES
 ('m1','original',1500,1500,'{"role":"assistant","cost":3,"modelID":"m","providerID":"p"}'),
 ('m2','copy',1500,1500,'{"role":"assistant","cost":3,"modelID":"m","providerID":"p"}'),
 ('m3','copy',2500,2500,'{"role":"assistant","cost":1,"modelID":"n","providerID":"q"}');`)
	// Independent construction oracle: two real calls cost 3 + 1 = 4;
	// the clone is not a third call. Existing legacy behavior stays 7.
	if got := period(t, path); got != 7 {
		t.Fatalf("legacy period behavior changed: got %v, want 7 (actual calls 4)", got)
	}
	// A pre-session timestamp can flag ambiguity, but import semantics must be
	// established before using it as an automatic exclusion rule.
	var copied int
	if err := db.QueryRow(`SELECT count(*) FROM message m JOIN session s ON s.id=m.session_id
 WHERE m.time_created < s.time_created`).Scan(&copied); err != nil || copied != 1 {
		t.Fatalf("ambiguous pre-session rows=%d, err=%v", copied, err)
	}
}

func TestHistoryV1CorrectionWithoutSessionTimestamp(t *testing.T) {
	path, db := fixture(t)
	execute(t, db, `INSERT INTO session VALUES ('s','Synthetic','/synthetic',1000,3000,3,NULL);
 INSERT INTO message VALUES ('m','s',1500,1500,'{"role":"assistant","cost":3}')`)
	if got := period(t, path); got != 3 {
		t.Fatalf("before correction=%v", got)
	}
	// Models updateMessage's conflict update: data changes independently of
	// session.time_updated. This is a possible write, not a native CLI run.
	execute(t, db, `UPDATE message SET data='{"role":"assistant","cost":1}' WHERE id='m'`)
	if got := period(t, path); got != 1 {
		t.Fatalf("after correction=%v", got)
	}
	var updated int64
	if err := db.QueryRow(`SELECT time_updated FROM session WHERE id='s'`).Scan(&updated); err != nil || updated != 3000 {
		t.Fatalf("session timestamp=%d, err=%v", updated, err)
	}
}

func TestHistorySnapshotAcrossWriterAndMigration(t *testing.T) {
	path, writer := fixture(t)
	execute(t, writer, `INSERT INTO session VALUES ('s','Synthetic','/synthetic',1000,3000,3,NULL)`)
	reader := openReader(t, path)
	tx, err := reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	var before, during int
	if err := tx.QueryRow(`SELECT count(*) FROM session`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	// Deterministic interleaving: a second WAL connection commits while the
	// read snapshot remains open. No sleeps, real user data, or native action.
	execute(t, writer, `DELETE FROM session;
 CREATE TABLE session_v2 (id TEXT PRIMARY KEY)`)
	if err := tx.QueryRow(`SELECT count(*) FROM session`).Scan(&during); err != nil || before != 1 || during != before {
		t.Fatalf("snapshot mixed source states: before=%d during=%d err=%v", before, during, err)
	}
	var tables int
	if err := tx.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='session_v2'`).Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("migration leaked into snapshot: tables=%d err=%v", tables, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='session_v2'`).Scan(&tables); err != nil || tables != 1 {
		t.Fatalf("new snapshot missed migration: tables=%d err=%v", tables, err)
	}
	if _, err := reader.Exec(`INSERT INTO session VALUES ('bad','Bad','/',1,1,0,NULL)`); err == nil {
		t.Fatal("experiment source connection was writable")
	}
}
