package research

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

const experimentV2Schema = `CREATE TABLE session_v2(id TEXT PRIMARY KEY,time_created INTEGER,fork_session_id TEXT,parent_id TEXT);
CREATE TABLE session_message(id TEXT PRIMARY KEY,session_id TEXT,type TEXT,time_created INTEGER,data TEXT);
CREATE TABLE event(id TEXT PRIMARY KEY,aggregate_id TEXT,type TEXT,created INTEGER,data TEXT);
CREATE INDEX message_session_type ON session_message(session_id,type);
CREATE INDEX event_aggregate_type ON event(aggregate_id,type);`

func readFixture(t *testing.T, path string, opts extractOptions) *Observation {
	t.Helper()
	o, err := extract(context.Background(), openReader(t, path), opts)
	if err != nil {
		t.Fatal(err)
	}
	return o
}
func payload(v2 bool, provider, model, variant string, cost any) string {
	fields := map[string]any{"role": "assistant", "cost": cost, "tokens": map[string]any{"input": 10, "output": 5, "reasoning": 2, "cache": map[string]int{"read": 3, "write": 4}}}
	if v2 {
		fields["model"] = map[string]string{"providerID": provider, "id": model, "variant": variant}
	} else {
		fields["providerID"] = provider
		fields["modelID"] = model
		fields["variant"] = variant
	}
	b, _ := json.Marshal(fields)
	return string(b)
}
func insertUsage(t *testing.T, db *sql.DB, v2 bool, id, session string, at any, data string) {
	t.Helper()
	if v2 {
		execute(t, db, `INSERT INTO session_message VALUES(?,?,'assistant',?,?)`, id, session, at, data)
	} else {
		execute(t, db, `INSERT INTO message VALUES(?,?,?,?,?)`, id, session, at, at, data)
	}
}
func addSession(t *testing.T, db *sql.DB, v2 bool, id string) {
	t.Helper()
	if v2 {
		execute(t, db, `INSERT INTO session_v2 VALUES(?,1000,NULL,NULL)`, id)
	} else {
		execute(t, db, `INSERT INTO session VALUES(?, 'Synthetic','/synthetic',1000,3000,0,NULL)`, id)
	}
}
func total(o *Observation) float64 {
	var n float64
	for _, u := range o.Items {
		if u.Cost != nil {
			n += *u.Cost
		}
	}
	return n
}
func digest(t *testing.T, db *sql.DB) string {
	t.Helper()
	h := sha256.New()
	for _, table := range []string{"sqlite_master", "session", "message", "session_v2", "session_message", "event"} {
		var found int
		if table != "sqlite_master" {
			if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name=?`, table).Scan(&found); err != nil {
				t.Fatal(err)
			}
			if found == 0 {
				continue
			}
		}
		rows, err := db.Query(`SELECT * FROM ` + table + ` ORDER BY rowid`)
		if err != nil {
			t.Fatal(err)
		}
		cols, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			v := make([]any, len(cols))
			p := make([]any, len(cols))
			for i := range v {
				p[i] = &v[i]
			}
			if err := rows.Scan(p...); err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(v)
			_, _ = h.Write(b)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		_ = rows.Close()
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestHistoryExtractDayModelAndQuality(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		t.Run(fmt.Sprint(v2), func(t *testing.T) {
			path, db := fixture(t)
			if v2 {
				execute(t, db, experimentV2Schema)
			}
			for _, id := range []string{"root", "child", "empty"} {
				addSession(t, db, v2, id)
			}
			if v2 {
				execute(t, db, `UPDATE session_v2 SET parent_id='root' WHERE id='child'`)
			} else {
				execute(t, db, `UPDATE session SET parent_id='root' WHERE id='child'`)
			}
			day1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).UnixMilli()
			day2 := day1 + 86400000
			insertUsage(t, db, v2, "a", "root", day1, payload(v2, "p", "m", "fast", 1.25))
			insertUsage(t, db, v2, "b", "root", day2, payload(v2, "q", "n", "", 2.5))
			insertUsage(t, db, v2, "c", "child", day1, payload(v2, "p", "m", "fast", .75))
			insertUsage(t, db, v2, "orphan", "absent", day1, payload(v2, "p", "m", "", 99))
			if v2 {
				execute(t, db, `INSERT INTO session_message VALUES('user','root','user',?,'{"cost":99}')`, day1)
			} else {
				insertUsage(t, db, false, "user", "root", day1, `{"role":"user","cost":99}`)
			}
			before := digest(t, db)
			o := readFixture(t, path, extractOptions{})
			after := digest(t, db)
			if before != after {
				t.Fatal("read changed source logical data/schema")
			}
			quality := ""
			policy := v2Policy
			if !v2 {
				quality = v1Warning
				policy = v1Policy
			}
			want := map[bucketKey]bucket{
				{"2026-01-01", "p", "m", "fast"}: {Cost: 2, KnownCost: 2, Count: 2, Tokens: [5]int64{20, 10, 4, 6, 8}, Quality: quality},
				{"2026-01-02", "q", "n", ""}:     {Cost: 2.5, KnownCost: 1, Count: 1, Tokens: [5]int64{10, 5, 2, 3, 4}, Quality: quality},
			}
			got := aggregate(o, time.UTC, nil, nil)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("buckets=%+v want=%+v", got, want)
			}
			if !o.Complete || len(o.Sessions) != 3 || !o.Sessions["empty"] || o.Sessions["absent"] || o.Policy != policy || len(o.Diagnostics) != 0 {
				t.Fatalf("observation=%+v", o)
			}
		})
	}
}
func TestHistoryExtractAcceptedV1Fork(t *testing.T) {
	path, db := fixture(t)
	addSession(t, db, false, "original")
	addSession(t, db, false, "copy")
	execute(t, db, `UPDATE session SET time_created=2000 WHERE id='copy'`)
	for _, id := range []string{"original", "copy"} {
		insertUsage(t, db, false, id, id, 1500, payload(false, "p", "m", "", 3))
	}
	insertUsage(t, db, false, "new", "copy", 2500, payload(false, "q", "n", "", 1))
	o := readFixture(t, path, extractOptions{})
	got := aggregate(o, time.UTC, nil, nil)
	if total(o) != 7 || got[bucketKey{"1970-01-01", "p", "m", ""}].Cost != 6 || got[bucketKey{"1970-01-01", "q", "n", ""}].Cost != 1 || o.Quality != v1Warning {
		t.Fatalf("accepted recorded-usage oracle failed: %+v", got)
	}
	// Independent actual-call reference stays 4, not 7.
	if total(o) == 4 {
		t.Fatal("silently deduplicated V1 fork")
	}
}
func TestHistoryExtractV2PeriodOracle(t *testing.T) {
	path, db := fixture(t)
	execute(t, db, experimentV2Schema)
	for _, id := range []string{"root", "child", "fork"} {
		addSession(t, db, true, id)
	}
	execute(t, db, `UPDATE session_v2 SET parent_id='root' WHERE id='child'; UPDATE session_v2 SET fork_session_id='root',time_created=2000 WHERE id='fork'`)
	for _, r := range []struct {
		id, s string
		at    int64
		cost  float64
	}{
		{"start", "root", 1000, 1}, {"child", "child", 1500, 2}, {"copied", "fork", 1500, 99}, {"new", "fork", 2000, 3}, {"end", "root", 3000, 99}, {"before", "root", 999, 99}, {"deleted", "absent", 1500, 99},
	} {
		insertUsage(t, db, true, r.id, r.s, r.at, payload(true, "p", "m", "", r.cost))
	}
	execute(t, db, `INSERT INTO session_message VALUES('compaction','root','compaction',1500,'{"cost":0.5}');
 INSERT INTO event VALUES
 ('e1','root','session.usage.recorded.1',1500,'{"source":"compaction","cost":0.5}'),
 ('e2','root','session.usage.recorded.1',1500,'{"source":"title","cost":99}'),
 ('e3','root','session.compaction.ended.1',1500,'{"cost":0.5}'),
 ('e4','absent','session.usage.recorded.1',1500,'{"source":"compaction","cost":99}'),
 ('e5','root','session.usage.recorded.1',3000,'{"source":"compaction","cost":99}')`)
	// Frozen legacy rows exist but have no authority.
	addSession(t, db, false, "legacy")
	insertUsage(t, db, false, "frozen", "legacy", 1500, payload(false, "p", "m", "", 99))
	o := readFixture(t, path, extractOptions{})
	for _, r := range []struct {
		from, to int64
		want     float64
	}{{1000, 3000, 6.5}, {2000, 3000, 3}, {4000, 5000, 0}, {1000, 1000, 0}} {
		var got float64
		for _, b := range aggregate(o, time.UTC, &r.from, &r.to) {
			got += b.Cost
		}
		if got != r.want {
			t.Fatalf("[%d,%d)=%v want %v", r.from, r.to, got, r.want)
		}
	}
	if _, ok := o.Items["root\x00event\x00e1"]; !ok {
		t.Fatal("missing compaction event")
	}
	if o.Diagnostics["unknown_model"] != 2 {
		t.Fatalf("unknown compaction models=%v", o.Diagnostics)
	}
}
func TestHistoryExtractDiagnostics(t *testing.T) {
	path, db := fixture(t)
	execute(t, db, experimentV2Schema)
	addSession(t, db, true, "s")
	for i, data := range []string{`{"cost":0}`, `{}`, `{"cost":null}`, `{"cost":"3"}`, `{"cost":-1}`, `{"cost":1e400}`, `{"cost":true}`, `broken`} {
		insertUsage(t, db, true, fmt.Sprint(i), "s", 1500, data)
	}
	insertUsage(t, db, true, "no-time", "s", nil, payload(true, "p", "m", "", 5))
	o := readFixture(t, path, extractOptions{})
	want := map[string]int{"unknown_cost": 6, "unknown_model": 7, "unknown_token_counter": 35, "missing_timestamp": 1, "malformed_json": 1}
	if !reflect.DeepEqual(o.Diagnostics, want) {
		t.Fatalf("diagnostics=%v want=%v", o.Diagnostics, want)
	}
	if len(o.Items) != 8 || o.Items["s\x00message\x000"].Cost == nil || !o.Complete {
		t.Fatalf("known zero or completeness lost: %+v", o)
	}
	if aggregate(o, time.UTC, nil, nil)[bucketKey{Provider: "p", Model: "m"}].Cost != 5 {
		t.Fatal("known cost without timestamp lost instead of reported as unbucketed")
	}
}
func TestHistoryExtractUnsupported(t *testing.T) {
	for _, mutation := range []string{`DROP TABLE message`, `ALTER TABLE message RENAME COLUMN data TO other`, `CREATE TABLE session_v2(id TEXT)`, experimentV2Schema + `DROP TABLE event`} {
		t.Run(mutation, func(t *testing.T) {
			path, db := fixture(t)
			execute(t, db, mutation)
			o, err := extract(context.Background(), openReader(t, path), extractOptions{})
			if err == nil || o != nil {
				t.Fatalf("unsupported schema returned publishable observation=%+v err=%v", o, err)
			}
		})
	}
}
func TestHistoryExtractInterruptionAndWriter(t *testing.T) {
	path, db := fixture(t)
	addSession(t, db, false, "s")
	for _, id := range []string{"a", "b", "c"} {
		insertUsage(t, db, false, id, "s", 1500, payload(false, "p", "m", "", 1))
	}
	old := readFixture(t, path, extractOptions{})
	for _, bounded := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		opts := extractOptions{}
		if bounded {
			opts.MaxRows = 1
		} else {
			opts.AfterItem = func(n int) {
				if n == 1 {
					cancel()
				}
			}
		}
		next, err := extract(ctx, openReader(t, path), opts)
		cancel()
		if err == nil || next != nil {
			t.Fatal("partial extraction publishable")
		}
		kept, state := reconcile(old, next, nil)
		if kept != old || state != "incomplete" {
			t.Fatal("interruption lost accepted observation")
		}
	}
	next := readFixture(t, path, extractOptions{AfterSnapshot: func() { execute(t, db, `DELETE FROM message WHERE id='a';`+experimentV2Schema) }})
	if next.Schema != "session" || total(next) != 3 {
		t.Fatalf("mixed snapshot %+v", next)
	}
	newest := readFixture(t, path, extractOptions{})
	if newest.Schema != "session_v2" || len(newest.Items) != 0 {
		t.Fatal("empty V2 did not win")
	}
	kept, state := reconcile(old, newest, nil)
	if kept != old || state != "unresolved-migration" {
		t.Fatal("migration silently cleared V1 quality or usage")
	}
}
func TestHistoryCalendarBoundaries(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct{ stamp, utc, local string }{
		{"2026-03-29T00:30:00Z", "2026-03-29", "2026-03-29"},
		{"2026-03-29T01:30:00Z", "2026-03-29", "2026-03-29"},
		{"2026-10-25T00:30:00Z", "2026-10-25", "2026-10-25"},
		{"2026-10-25T01:30:00Z", "2026-10-25", "2026-10-25"},
		{"2026-12-31T23:30:00Z", "2026-12-31", "2027-01-01"},
		{"2026-01-31T23:30:00Z", "2026-01-31", "2026-02-01"},
	} {
		at, err := time.Parse(time.RFC3339, r.stamp)
		if err != nil {
			t.Fatal(err)
		}
		cost := 1.0
		o := &Observation{Items: map[string]Usage{"u": {At: at.UnixMilli(), TimeKnown: true, Cost: &cost}}}
		for _, z := range []struct {
			zone *time.Location
			day  string
		}{{time.UTC, r.utc}, {berlin, r.local}} {
			got := aggregate(o, z.zone, nil, nil)
			if len(got) != 1 || got[bucketKey{Day: z.day}].Cost != 1 {
				t.Fatalf("%s zone=%v buckets=%v", r.stamp, z.zone, got)
			}
		}
	}
}
func TestHistoryConservativeReconciliation(t *testing.T) {
	path, db := fixture(t)
	addSession(t, db, false, "s")
	insertUsage(t, db, false, "a", "s", 1500, payload(false, "p", "m", "", 3))
	old := readFixture(t, path, extractOptions{})
	execute(t, db, `UPDATE message SET data=? WHERE id='a'`, payload(false, "q", "n", "", 1))
	next := readFixture(t, path, extractOptions{})
	kept, state := reconcile(old, next, nil)
	if kept != old || state != "unresolved-correction" {
		t.Fatal("stable ID treated as correction authority")
	}
	kept, state = reconcile(old, next, map[string]bool{"s\x00message\x00a": true})
	if kept != next || state != "accepted" || total(kept) != 1 || kept.Quality != v1Warning {
		t.Fatal("explicit fixture correction authority failed")
	}
	// Reattribution and zero replacement require the same explicit evidence.
	execute(t, db, `UPDATE message SET data=? WHERE id='a'`, payload(false, "q", "n", "", 0))
	zero := readFixture(t, path, extractOptions{})
	kept, state = reconcile(next, zero, map[string]bool{"s\x00message\x00a": true})
	if kept != zero || total(kept) != 0 || state != "accepted" {
		t.Fatal("verified zero correction failed")
	}
	execute(t, db, `DELETE FROM message; INSERT INTO message VALUES('replacement','s',1500,1500,?)`, payload(false, "p", "m", "", 2))
	lost := readFixture(t, path, extractOptions{})
	kept, state = reconcile(old, lost, map[string]bool{"s\x00message\x00a": true})
	if kept != old || !strings.HasPrefix(state, "unresolved") || total(kept) != 3 {
		t.Fatal("loss/replacement mixed old/new contributions")
	}
	execute(t, db, `DELETE FROM session`)
	missing := readFixture(t, path, extractOptions{})
	kept, state = reconcile(old, missing, nil)
	if kept != old || state != "unresolved-record-loss" {
		t.Fatal("source deletion erased captured usage")
	}
	// Serialize accounting-only state to simulate retention/restart with no source.
	raw, err := json.Marshal(kept)
	if err != nil {
		t.Fatal(err)
	}
	var restarted Observation
	if err = json.Unmarshal(raw, &restarted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(aggregate(kept, time.UTC, nil, nil), aggregate(&restarted, time.UTC, nil, nil)) || restarted.Quality != v1Warning {
		t.Fatal("restart lost quality/usage")
	}
	for _, forbidden := range []string{"Synthetic", "/synthetic", "role", "content"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("retained content: %s", forbidden)
		}
	}
}

func TestHistoryCandidatePersistence(t *testing.T) {
	path, db := fixture(t)
	addSession(t, db, false, "s")
	insertUsage(t, db, false, "m", "s", 1500, payload(false, "p", "m", "", 7))
	o := readFixture(t, path, extractOptions{})
	destPath, dest := fixture(t)
	execute(t, dest, candidateSchema)
	for i := 0; i < 2; i++ {
		if err := writeCandidate(context.Background(), dest, o); err != nil {
			t.Fatal(err)
		}
	}
	execute(t, db, `DELETE FROM message; DELETE FROM session`)
	reader := openReader(t, destPath)
	var cost float64
	var count int
	var quality, policy string
	if err := reader.QueryRow(`SELECT sum(cost),count(*),min(quality),min(policy) FROM usage`).Scan(&cost, &count, &quality, &policy); err != nil {
		t.Fatal(err)
	}
	if cost != 7 || count != 1 || quality != v1Warning || policy != v1Policy {
		t.Fatal("restart/source deletion/retry lost quality or duplicated contribution")
	}
	if err := reader.QueryRow(`SELECT sum(cost),sum(steps),min(quality) FROM daily`).Scan(&cost, &count, &quality); err != nil {
		t.Fatal(err)
	}
	if cost != 7 || count != 1 || quality != v1Warning {
		t.Fatal("saved day/range quality missing")
	}
	// A failed destination write rolls back all deletions/inserts together.
	execute(t, dest, `CREATE TRIGGER reject_daily BEFORE INSERT ON daily BEGIN SELECT RAISE(FAIL,'fixture failure'); END`)
	if err := writeCandidate(context.Background(), dest, o); err == nil {
		t.Fatal("injected write failure ignored")
	}
	if err := reader.QueryRow(`SELECT sum(cost),count(*) FROM usage`).Scan(&cost, &count); err != nil || cost != 7 || count != 1 {
		t.Fatalf("failed write lost old contribution: %v %d %v", cost, count, err)
	}
}

func TestHistoryV2ReplacementLoss(t *testing.T) {
	path, db := fixture(t)
	execute(t, db, experimentV2Schema)
	addSession(t, db, true, "s")
	insertUsage(t, db, true, "original", "s", 1500, payload(true, "p", "m", "", 3))
	old := readFixture(t, path, extractOptions{})
	execute(t, db, `DELETE FROM session_message;
 INSERT INTO session_message VALUES('projection','s','compaction',2000,'{"cost":3}');
 INSERT INTO event VALUES('replacement','s','session.usage.recorded.1',2000,'{"source":"compaction","cost":3}')`)
	next := readFixture(t, path, extractOptions{})
	if total(next) != 3 || len(next.Items) != 1 {
		t.Fatal("double counted compaction projection and event")
	}
	kept, state := reconcile(old, next, nil)
	if kept != old || !strings.HasPrefix(state, "unresolved") || len(kept.Items) != 1 {
		t.Fatal("ambiguous replacement mixed potentially overlapping usage")
	}
}

func TestHistoryQueryPlan(t *testing.T) {
	_, db := fixture(t)
	rows, err := db.Query(`EXPLAIN QUERY PLAN SELECT m.id,m.session_id,m.time_created FROM message m JOIN session s ON s.id=m.session_id ORDER BY m.session_id,m.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		t.Log(detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
