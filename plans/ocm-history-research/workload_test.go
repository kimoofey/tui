package research

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Synthetic shapes only. No installed OpenCode executable or source is used.
type workload struct {
	Name           string
	Rows, Sessions int
	V2             bool
}

var scaleWorkloads = []workload{
	{"v1-10k", 10000, 100, false}, {"v2-10k", 10000, 100, true},
	{"v1-100k", 100000, 1000, false}, {"v2-100k", 100000, 1000, true},
	{"v1-huge", 100000, 1, false}, {"v2-huge", 100000, 1, true},
}

const workloadEpoch int64 = 1767225600000 // 2026-01-01 UTC
const workloadV2Schema = `CREATE TABLE session_v2 (
 id TEXT PRIMARY KEY,title TEXT,directory TEXT NOT NULL,time_created INTEGER,
 time_updated INTEGER,cost REAL,parent_id TEXT,fork_session_id TEXT);
CREATE TABLE session_message (
 id TEXT PRIMARY KEY,session_id TEXT,type TEXT,seq INTEGER,time_created INTEGER,
 time_updated INTEGER,data TEXT);
CREATE TABLE event (
 id TEXT PRIMARY KEY,aggregate_id TEXT,seq INTEGER,type TEXT,data TEXT,created INTEGER);
CREATE INDEX session_message_session_seq_idx ON session_message(session_id,seq);
CREATE INDEX session_message_session_time_created_id_idx ON session_message(session_id,time_created,id);
CREATE INDEX session_message_session_type_seq_idx ON session_message(session_id,type,seq);
CREATE INDEX session_message_time_created_idx ON session_message(time_created);
CREATE INDEX event_aggregate_type ON event(aggregate_id,type);`

func workloadDB(t testing.TB, path, mode string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=" + mode}).String())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func requireSQL(t testing.TB, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}

func workloadJSON(w workload, i, padding int, cost float64) string {
	data := payload(w.V2, fmt.Sprintf("p%d", i%2), fmt.Sprintf("m%d", i%8), fmt.Sprintf("v%d", i%2), cost)
	return strings.TrimSuffix(data, "}") + `,"content":"` + strings.Repeat("x", padding) + `"}`
}

func generateWorkload(t testing.TB, w workload) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic.db")
	db := workloadDB(t, path, "rwc")
	requireSQL(t, db, `PRAGMA journal_mode=WAL`)
	requireSQL(t, db, `PRAGMA wal_autocheckpoint=0`)
	requireSQL(t, db, legacySchema)
	if w.V2 {
		requireSQL(t, db, workloadV2Schema)
		// Frozen V1 data must not contribute, even in capacity experiments.
		requireSQL(t, db, `INSERT INTO session VALUES('frozen','Synthetic','/synthetic',1,1,999,NULL);
INSERT INTO message VALUES('frozen','frozen',1,1,'{"role":"assistant","cost":999}')`)
	}
	requireSQL(t, db, `CREATE TABLE history_workload(name TEXT NOT NULL)`)
	requireSQL(t, db, `INSERT INTO history_workload VALUES(?)`, w.Name)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	for s := range w.Sessions {
		var parent, fork any
		if s > 0 && s%10 == 0 {
			parent = "s000000"
		}
		q := `INSERT INTO session VALUES(?,?,?,?,?,0,?)`
		args := []any{fmt.Sprintf("s%06d", s), fmt.Sprintf("Synthetic %06d", s), "/synthetic", workloadEpoch, workloadEpoch, parent}
		if w.V2 {
			if s == 1 {
				fork = "s000000"
			}
			q = `INSERT INTO session_v2 VALUES(?,?,?,?,?,0,?,?)`
			args = append(args, fork)
		}
		if _, err = tx.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	messageSQL := `INSERT INTO message VALUES(?,?,?,?,?)`
	if w.V2 {
		messageSQL = `INSERT INTO session_message VALUES(?,?,?,?,?,?,?)`
	}
	message, err := tx.Prepare(messageSQL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = message.Close() }()
	insert := func(id, sid, kind string, seq int, at int64, data string) {
		args := []any{id, sid, at, at, data}
		if w.V2 {
			args = []any{id, sid, kind, seq, at, at, data}
		}
		if _, e := message.Exec(args...); e != nil {
			t.Fatal(e)
		}
	}
	// Reuse synthetic strings while generating; fixture allocations are excluded
	// from fresh worker RSS. SQLite still parses the complete containing JSON.
	cache := map[[2]int]string{}
	for i := range w.Rows {
		padding := []int{128, 1024, 8192}[i%3]
		if i%8192 == 0 {
			padding = 3 << 20
		}
		key := [2]int{i % 8, padding}
		data, ok := cache[key]
		if !ok {
			data = workloadJSON(w, i, padding, .125)
			cache[key] = data
		}
		sid, at := fmt.Sprintf("s%06d", i%w.Sessions), workloadEpoch+int64(i%31)*86400000
		insert(fmt.Sprintf("m%09d", i), sid, "assistant", i*3, at, data)
		if i%100 == 0 {
			insert(fmt.Sprintf("u%09d", i), sid, "user", i*3+1, at, `{"role":"user","cost":999}`)
			if w.V2 {
				data := strings.TrimSuffix(workloadJSON(w, i, 0, .25), "}") + `,"source":"compaction"}`
				insert(fmt.Sprintf("c%09d", i), sid, "compaction", i*3+2, at, data)
				if _, err = tx.Exec(`INSERT INTO event VALUES(?,?,?,'session.usage.recorded.1',?,?)`, fmt.Sprintf("e%09d", i), sid, i, data, at); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if w.Sessions > 1 {
		// V1 counts a distinct copied ID; V2 excludes this pre-fork projection.
		insert("copy", "s000001", "assistant", -1, workloadEpoch-1, workloadJSON(w, 0, 128, .125))
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	requireSQL(t, db, `PRAGMA wal_checkpoint(TRUNCATE)`)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// The oracle follows fixture construction, without parsing source JSON or using
// the extractor's timestamps, aggregate(), SQL queries, or candidate rows.
func workloadOracle(w workload) map[bucketKey]bucket {
	out := map[bucketKey]bucket{}
	add := func(i int, day string, cost float64) {
		key := bucketKey{day, fmt.Sprintf("p%d", i%2), fmt.Sprintf("m%d", i%8), fmt.Sprintf("v%d", i%2)}
		b := out[key]
		b.Count++
		b.KnownCost++
		b.Cost += cost
		if !w.V2 {
			b.Quality = v1Warning
		}
		for j, tokens := range [5]int64{10, 5, 2, 3, 4} {
			b.Tokens[j] += tokens
		}
		out[key] = b
	}
	for i := range w.Rows {
		day := fmt.Sprintf("2026-01-%02d", 1+i%31)
		add(i, day, .125)
		if w.V2 && i%100 == 0 {
			add(i, day, .25)
		}
	}
	if !w.V2 && w.Sessions > 1 {
		add(0, "2025-12-31", .125)
	}
	return out
}

func checkWorkload(t testing.TB, w workload, o *Observation, archive *sql.DB) {
	t.Helper()
	want := workloadOracle(w)
	if !o.Complete || len(o.Sessions) != w.Sessions || len(o.Diagnostics) != 0 || !reflect.DeepEqual(aggregate(o, time.UTC, nil, nil), want) {
		t.Fatal("workload observation disagrees with independent day/model/token/quality oracle")
	}
	count, cost := 0, 0.0
	for _, b := range want {
		count += b.Count
		cost += b.Cost
	}
	if len(o.Items) != count {
		t.Fatal("incorrect usage membership")
	}
	if archive == nil {
		return
	}
	var gotCount, dailyCount, known, unknown int
	var gotCost, dailyCost float64
	if err := archive.QueryRow(`SELECT count(*),sum(cost) FROM usage`).Scan(&gotCount, &gotCost); err != nil {
		t.Fatal(err)
	}
	if err := archive.QueryRow(`SELECT sum(steps),sum(known_cost),sum(unknown_cost),sum(cost) FROM daily`).Scan(&dailyCount, &known, &unknown, &dailyCost); err != nil {
		t.Fatal(err)
	}
	if gotCount != count || dailyCount != count || known != count || unknown != 0 || gotCost != cost || dailyCost != cost {
		t.Fatal("retained accounting disagrees with fixture construction")
	}
	var tokens [5]int64
	if err := archive.QueryRow(`SELECT sum(json_extract(tokens,'$[0]')),
sum(json_extract(tokens,'$[1]')),sum(json_extract(tokens,'$[2]')),
sum(json_extract(tokens,'$[3]')),sum(json_extract(tokens,'$[4]')) FROM usage`).Scan(
		&tokens[0], &tokens[1], &tokens[2], &tokens[3], &tokens[4]); err != nil {
		t.Fatal(err)
	}
	for i, each := range [5]int64{10, 5, 2, 3, 4} {
		if tokens[i] != each*int64(count) {
			t.Fatal("retained token counters disagree with fixture construction")
		}
	}
	rows, err := archive.Query(`SELECT day,provider,model,variant,cost,known_cost,unknown_cost,steps,quality FROM daily`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	n := 0
	for rows.Next() {
		var k bucketKey
		var b bucket
		if err = rows.Scan(&k.Day, &k.Provider, &k.Model, &k.Variant, &b.Cost, &b.KnownCost, &b.UnknownCost, &b.Count, &b.Quality); err != nil {
			t.Fatal(err)
		}
		expected, ok := want[k]
		if !ok || b.Cost != expected.Cost || b.Count != expected.Count || b.KnownCost != expected.KnownCost || b.UnknownCost != expected.UnknownCost || b.Quality != expected.Quality {
			t.Fatal("retained day/model/quality differs from oracle")
		}
		n++
	}
	if err = rows.Err(); err != nil || n != len(want) {
		t.Fatalf("daily iteration: rows=%d err=%v", n, err)
	}
}

func TestHistoryGeneratedWorkloads(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		t.Run(fmt.Sprint(v2), func(t *testing.T) {
			w := workload{"small", 120, 4, v2}
			source := openReader(t, generateWorkload(t, w))
			before := digest(t, source)
			o, err := extract(context.Background(), source, extractOptions{})
			if err != nil {
				t.Fatal(err)
			}
			dest := workloadDB(t, filepath.Join(t.TempDir(), "candidate.db"), "rwc")
			requireSQL(t, dest, candidateSchema)
			for range 2 {
				if err = writeCandidate(context.Background(), dest, o); err != nil {
					t.Fatal(err)
				}
				checkWorkload(t, w, o, dest)
			}
			if digest(t, source) != before {
				t.Fatal("source logical data or schema changed")
			}
		})
	}
}
