package research

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func pipelineArchive(t testing.TB) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.db")
	db := workloadDB(t, path, "rwc")
	requireSQL(t, db, `PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; PRAGMA foreign_keys=ON; PRAGMA wal_autocheckpoint=1000;`)
	requireSQL(t, db, pipelineSchema)
	return db, path
}
func stageFactory(t testing.TB) func() (*sql.DB, string, error) {
	return func() (*sql.DB, string, error) { s := newStage(t); return s.db, s.path, nil }
}
func acceptedCost(t testing.TB, db *sql.DB) float64 {
	t.Helper()
	var v float64
	if err := db.QueryRow(`SELECT coalesce(sum(cost),0) FROM session_daily`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}
func assertPipelineOracle(t testing.TB, w workload, db *sql.DB) {
	t.Helper()
	rows, err := db.Query(`SELECT day,provider,model,variant,cost,known_cost,unknown_cost,steps,tokens,unknown_tokens,quality FROM session_daily`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	got := map[bucketKey]bucket{}
	for rows.Next() {
		var k bucketKey
		var b bucket
		var tokens, unknown string
		if err = rows.Scan(&k.Day, &k.Provider, &k.Model, &k.Variant, &b.Cost, &b.KnownCost, &b.UnknownCost, &b.Count, &tokens, &unknown, &b.Quality); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal([]byte(tokens), &b.Tokens); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal([]byte(unknown), &b.UnknownTokens); err != nil {
			t.Fatal(err)
		}
		a := got[k]
		a.Cost += b.Cost
		a.Count += b.Count
		a.KnownCost += b.KnownCost
		a.UnknownCost += b.UnknownCost
		a.Quality = b.Quality
		for j := range a.Tokens {
			a.Tokens[j] += b.Tokens[j]
			a.UnknownTokens[j] += b.UnknownTokens[j]
		}
		got[k] = a
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, workloadOracle(w)) {
		t.Fatal("published archive differs from independent fixture oracle")
	}
}

func TestHistoryPipelineSafety(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		t.Run(fmt.Sprint(v2), func(t *testing.T) {
			w := workload{"pipeline-safety", 120, 4, v2}
			path := generateWorkload(t, w)
			source := openReader(t, path)
			archive, _ := pipelineArchive(t)
			ctx := context.Background()
			r, err := sweep(ctx, source, archive, stageFactory(t), pipelineHooks{})
			if err != nil || r.Sessions != 4 || r.Deferred != 0 {
				t.Fatal(r, err)
			}
			assertPipelineOracle(t, w, archive)
			o, err := extract(ctx, source, extractOptions{Session: "s000000"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = publishSession(ctx, archive, "s000000", o, nil, publishOptions{}); !errors.Is(err, errStalePublication) {
				t.Fatal("stale generation accepted", err)
			}
			want := acceptedCost(t, archive)
			key := "s000000\x00message\x00m000000000"
			u := o.Items[key]
			cost := 0.0
			u.Cost = &cost
			o.Items[key] = u
			status, err := publishSession(ctx, archive, "s000000", o, nil, publishOptions{Expected: 1})
			if err != nil || status != "unresolved-correction" || acceptedCost(t, archive) != want {
				t.Fatal(status, err)
			}
			cancelCtx, cancel := context.WithCancel(ctx)
			_, err = publishSession(cancelCtx, archive, "s000000", o, nil, publishOptions{Expected: 1, Corrections: map[string]bool{key: true}, BeforeCommit: func() error { cancel(); return nil }})
			if err == nil || acceptedCost(t, archive) != want {
				t.Fatal("cancelled replacement committed", err)
			}
			injected := errors.New("injected precommit failure")
			_, err = publishSession(ctx, archive, "s000000", o, nil, publishOptions{Expected: 1, Corrections: map[string]bool{key: true}, BeforeCommit: func() error { return injected }})
			if !errors.Is(err, injected) || acceptedCost(t, archive) != want {
				t.Fatal("failed replacement committed", err)
			}
			status, err = publishSession(ctx, archive, "s000000", o, nil, publishOptions{Expected: 1, Corrections: map[string]bool{key: true}})
			if err != nil || status != "accepted" || acceptedCost(t, archive) != want-.125 {
				t.Fatal(status, err)
			}
			delete(o.Items, key)
			status, err = publishSession(ctx, archive, "s000000", o, nil, publishOptions{Expected: 2})
			if err != nil || status != "unresolved-record-loss" || acceptedCost(t, archive) != want-.125 {
				t.Fatal(status, err)
			}
			// Unsealed stages cannot publish; stage records must match the sealed scope.
			s := newStage(t)
			if _, err = publishSession(ctx, archive, "s000000", nil, s.db, publishOptions{Expected: 2}); err == nil {
				t.Fatal("unsealed stage accepted")
			}
			if _, err = stageSession(ctx, source, s.db, extractOptions{Session: "s000001"}); err != nil {
				t.Fatal(err)
			}
			if _, err = publishSession(ctx, archive, "s000000", nil, s.db, publishOptions{Expected: 2}); err == nil {
				t.Fatal("foreign stage scope accepted")
			}
			// Publish a sealed stream and compare all daily/token fields to the oracle.
			fresh, _ := pipelineArchive(t)
			if _, err = publishSession(ctx, fresh, "s000001", nil, s.db, publishOptions{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHistoryPipelineInventory(t *testing.T) {
	path, writer := fixture(t)
	for _, id := range []string{"a", "b", "c"} {
		execute(t, writer, `INSERT INTO session VALUES(?,'Synthetic','/synthetic',1,1,0,NULL)`, id)
	}
	reader := openReader(t, path)
	ids, err := inventoryPage(context.Background(), reader, "", 2)
	if err != nil || !reflect.DeepEqual(ids, []string{"a", "b"}) {
		t.Fatal(ids, err)
	}
	execute(t, writer, `INSERT INTO session VALUES('d','Synthetic','/synthetic',1,1,0,NULL)`)
	ids, err = inventoryPage(context.Background(), reader, "b", 2)
	if err != nil || !reflect.DeepEqual(ids, []string{"c", "d"}) {
		t.Fatal(ids, err)
	}
	if _, err = inventoryPage(context.Background(), reader, "", 0); err == nil {
		t.Fatal("unbounded page accepted")
	}
}

func TestHistoryPipelineDiskFull(t *testing.T) {
	w := workload{"disk-full", 120, 4, false}
	source := openReader(t, generateWorkload(t, w))
	archive, _ := pipelineArchive(t)
	ctx := context.Background()
	if _, err := sweep(ctx, source, archive, stageFactory(t), pipelineHooks{}); err != nil {
		t.Fatal(err)
	}
	before := acceptedCost(t, archive)
	// SQLite's explicit page quota deterministically injects SQLITE_FULL without
	// filling the host disk or relying on filesystem permissions/root identity.
	s := newStage(t)
	requireSQL(t, s.db, `PRAGMA max_page_count=3`)
	_, err := stageSession(ctx, source, s.db, extractOptions{Session: "s000000"})
	if err == nil {
		t.Fatal("stage disk quota did not fail")
	}
	assertUnsealed(t, s.db)
	requireSQL(t, archive, `PRAGMA wal_checkpoint(TRUNCATE)`)
	var pages int
	if err = archive.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	requireSQL(t, archive, fmt.Sprintf(`PRAGMA max_page_count=%d`, pages))
	o, err := extract(ctx, source, extractOptions{Session: "s000000"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 4000 {
		u := o.Items["s000000\x00message\x00m000000000"]
		u.ID = fmt.Sprintf("new%06d", i)
		o.Items[usageKey(u)] = u
	}
	if _, err = publishSession(ctx, archive, "s000000", o, nil, publishOptions{Expected: 1}); err == nil {
		t.Fatal("archive disk quota did not fail")
	}
	if acceptedCost(t, archive) != before {
		t.Fatal("disk failure changed accepted accounting")
	}
}

func TestHistoryPipelineEmptySchema(t *testing.T) {
	path, writer := fixture(t)
	reader := openReader(t, path)
	ids, err := inventoryPage(context.Background(), reader, "", 100)
	if err != nil || len(ids) != 0 {
		t.Fatal(ids, err)
	}
	// V2 is authoritative even when empty and broken; never fall back to valid V1.
	execute(t, writer, `CREATE TABLE session_v2(id TEXT PRIMARY KEY,time_created INTEGER,fork_session_id TEXT)`)
	if _, err = inventoryPage(context.Background(), reader, "", 100); err == nil {
		t.Fatal("missing V2 usage tables treated as empty inventory")
	}
	execute(t, writer, `DROP TABLE session_v2; DROP TABLE message`)
	if _, err = inventoryPage(context.Background(), reader, "", 100); err == nil {
		t.Fatal("missing V1 usage table treated as empty inventory")
	}
}
