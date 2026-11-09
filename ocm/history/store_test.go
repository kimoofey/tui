package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kimoofey/tui/ocm/session"
)

func ptr[T any](v T) *T { return &v }
func observation() session.Observation {
	return session.Observation{Binding: session.HistoryBinding{Path: "/fixture/source.db", Schema: "session"}, SessionID: "root", Present: true, ApproximateV1: true, Usage: []session.Usage{{Kind: "message", ID: "m", Timestamp: ptr(int64(0)), Cost: ptr(2.0), Provider: ptr("p"), Model: ptr("m"), Input: ptr(int64(1))}}}
}
func openTest(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ocm", "history.sqlite3")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}
func report(t *testing.T, s *Store) Report {
	t.Helper()
	r, err := s.Query(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestReconciliation(t *testing.T) {
	s, path := openTest(t)
	ctx := context.Background()
	o := observation()
	capture := func() {
		t.Helper()
		if err := s.Capture(ctx, o, time.UnixMilli(123)); err != nil {
			t.Fatal(err)
		}
	}
	capture()
	capture()
	if r := report(t, s); r.Rows != 1 || *r.Cost != 2 {
		t.Fatal(r)
	}
	for _, amount := range []float64{5, 1, 0} {
		o.Usage[0].Cost = ptr(amount)
		o.Usage[0].Timestamp = ptr(int64(86400000))
		o.Usage[0].Model = ptr("changed")
		capture()
		if r := report(t, s); *r.Cost != amount || r.Days[0].UTC != "1970-01-02" || *r.Models[0].Name != "changed" {
			t.Fatal(r)
		}
	}
	o.Usage = append(o.Usage, session.Usage{Kind: "message", ID: "new", Cost: ptr(3.0)})
	capture()
	capture()
	old := o.Usage[0]
	o.Usage = o.Usage[1:]
	o.Usage = append(o.Usage, session.Usage{Kind: "message", ID: "unaccepted", Cost: ptr(90.0)})
	if err := s.Capture(ctx, o, time.Now()); !errors.Is(err, ErrStale) {
		t.Fatal(err)
	}
	if r := report(t, s); r.Rows != 2 || *r.Cost != 3 || r.Sessions[0].Status != ErrStale.Error() {
		t.Fatal(r)
	}
	o.Usage = []session.Usage{old, o.Usage[0]}
	o.Usage[0].Input = nil
	if err := s.Capture(ctx, o, time.Now()); !errors.Is(err, ErrStale) {
		t.Fatal(err)
	}
	o.Present = false
	if err := s.Capture(ctx, o, time.Now()); err == nil {
		t.Fatal("absent accepted")
	}
	if *report(t, s).Cost != 3 {
		t.Fatal("usage lost")
	}
	s.Close()
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if r := report(t, reopened); *r.Cost != 3 || !r.ApproximateV1 {
		t.Fatal(r)
	}
}
func TestAtomicityBindingAndUnknowns(t *testing.T) {
	s, _ := openTest(t)
	ctx := context.Background()
	o := observation()
	o.Usage = nil
	if err := s.Capture(ctx, o, time.Now()); err != nil {
		t.Fatal(err)
	}
	if r := report(t, s); len(r.Sessions) != 1 || r.Cost != nil {
		t.Fatal(r)
	}
	o.Usage = observation().Usage
	o.Binding.Path = "/different"
	if err := s.Capture(ctx, o, time.Now()); !errors.Is(err, ErrBinding) {
		t.Fatal(err)
	}
	o = observation()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.Capture(cancelled, o, time.Now()); err == nil {
		t.Fatal("cancellation accepted")
	}
	if report(t, s).Rows != 0 {
		t.Fatal("partial publish")
	}
	o.Usage = append(o.Usage, o.Usage[0])
	if err := s.Capture(ctx, o, time.Now()); err == nil {
		t.Fatal("duplicate accepted")
	}
	o.Usage = []session.Usage{{Kind: "message", ID: "unknown"}, {Kind: "message", ID: "zero", Cost: ptr(0.0), Input: ptr(int64(math.MaxInt64))}, {Kind: "message", ID: "extra", Input: ptr(int64(1))}}
	if err := s.Capture(ctx, o, time.Now()); err != nil {
		t.Fatal(err)
	}
	r := report(t, s)
	if r.Cost == nil || *r.Cost != 0 || r.UnknownCost != 2 || r.Unbucketed != 3 || r.UnknownModel != 3 || r.Tokens[0] != nil {
		t.Fatal(r)
	}
	dated, err := s.Query(ctx, &Range{Start: 0, End: 1})
	if err != nil || dated.Rows != 0 {
		t.Fatal(dated, err)
	}
	// A failed commit cannot replace accepted usage.
	_, err = s.db.Exec(`CREATE TRIGGER fail_usage BEFORE UPDATE ON usage BEGIN SELECT RAISE(ABORT,'fixture failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	o.Usage[1].Cost = ptr(50.0)
	if err = s.Capture(ctx, o, time.Now()); err == nil {
		t.Fatal("trigger should abort")
	}
	if *report(t, s).Cost != 0 {
		t.Fatal("failed transaction leaked")
	}
}
func TestArchiveVersionPermissionsAndLock(t *testing.T) {
	if path := os.Getenv("OCM_LOCK_HELPER"); path != "" {
		s, err := Open(path)
		if err != nil {
			os.Exit(17)
		}
		s.Close()
		os.Exit(0)
	}
	s, path := openTest(t)
	for _, p := range []string{path, path + ".lock", path + "-wal", path + "-shm"} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("%s: %o", p, info.Mode().Perm())
		}
	}
	child := func() error {
		cmd := exec.Command(os.Args[0], "-test.run=^TestArchiveVersionPermissionsAndLock$")
		cmd.Env = append(os.Environ(), "OCM_LOCK_HELPER="+path)
		return cmd.Run()
	}
	if err := child(); err == nil {
		t.Fatal("second process acquired lock")
	}
	s.Close()
	if err := child(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`PRAGMA user_version=99`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := Open(path); err == nil {
		s.Close()
		t.Fatal("newer archive accepted")
	}
	db, _ = sql.Open("sqlite", path)
	defer db.Close()
	var v int
	if err = db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil || v != 99 {
		t.Fatal(v, err)
	}
}
func TestConfig(t *testing.T) {
	for _, tc := range []struct {
		yaml          string
		enabled, fail bool
	}{{"", false, false}, {"{}", false, false}, {"history:\n  enabled: true", true, false}, {"history:\n  enabled: false", false, false}, {"history:\n  typo: true", false, true}, {"unknown: true", false, true}, {"history: [", false, true}, {"{}\n---\n{}", false, true}, {"history:\n  enabled: maybe", false, true}} {
		c, err := ReadConfig("fixture", func(string) ([]byte, error) { return []byte(tc.yaml), nil })
		if (err != nil) != tc.fail || c.History.Enabled != tc.enabled {
			t.Fatalf("%q: %+v %v", tc.yaml, c, err)
		}
	}
	c, err := ReadConfig("missing", func(string) ([]byte, error) { return nil, os.ErrNotExist })
	if err != nil || c.History.Enabled {
		t.Fatal(c, err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", "")
	p, err := ArchivePath()
	if err != nil || p != filepath.Join(os.Getenv("HOME"), ".local/share/ocm/history.sqlite3") {
		t.Fatal(p, err)
	}
}

func TestArchiveCorruptOrUnknownNeverReset(t *testing.T) {
	for _, mode := range []string{"corrupt", "unversioned", "policy"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "archive.db")
			if mode == "corrupt" {
				if err := os.WriteFile(path, []byte("not sqlite"), 0600); err != nil {
					t.Fatal(err)
				}
			} else if mode == "unversioned" {
				db, _ := sql.Open("sqlite", path)
				_, err := db.Exec(`CREATE TABLE unrelated(value TEXT); INSERT INTO unrelated VALUES('retained')`)
				db.Close()
				if err != nil {
					t.Fatal(err)
				}
			} else {
				s, err := Open(path)
				if err != nil {
					t.Fatal(err)
				}
				_, err = s.db.Exec(`INSERT INTO metadata VALUES(1,99,'/source','session')`)
				s.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if s, err := Open(path); err == nil {
				s.Close()
				t.Fatal("invalid archive accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("archive modified")
			}
		})
	}
}

func TestQuerySnapshotAcrossConcurrentCommit(t *testing.T) {
	s, path := openTest(t)
	ctx := context.Background()
	o := observation()
	if err := s.Capture(ctx, o, time.UnixMilli(123)); err != nil {
		t.Fatal(err)
	}
	writer, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	// The separate writer commits after the total query, before every other
	// component query. All components must still see the reader's original snapshot.
	r, err := s.querySnapshot(ctx, nil, func() {
		if _, err := writer.Exec(`BEGIN; UPDATE usage SET cost=9,model='new'; UPDATE sessions SET captured=999,status='changed'; COMMIT;`); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || *r.Cost != 2 || *r.Days[0].Cost != 2 || *r.Models[0].Cost != 2 || *r.Models[0].Name != "m" || r.Sessions[0].Captured != 123 || r.Sessions[0].Status != "" {
		t.Fatal(r, err)
	}
	if next := report(t, s); *next.Cost != 9 || next.Sessions[0].Captured != 999 {
		t.Fatal(next)
	}
}
func TestUTCRangesAndUnknownBuckets(t *testing.T) {
	s, _ := openTest(t)
	o := observation()
	o.Usage = nil
	for i, at := range []int64{-1, 0, 86400000} {
		u := observation().Usage[0]
		u.ID = fmt.Sprint(i)
		u.Timestamp = ptr(at)
		o.Usage = append(o.Usage, u)
	}
	u := observation().Usage[0]
	u.ID = "unbucketed"
	u.Timestamp = nil
	o.Usage = append(o.Usage, u)
	if err := s.Capture(context.Background(), o, time.Now()); err != nil {
		t.Fatal(err)
	}
	r, err := s.Query(context.Background(), &Range{Start: 0, End: 86400000})
	if err != nil || r.Rows != 1 || len(r.Days) != 1 || r.Days[0].UTC != "1970-01-01" || r.Unbucketed != 0 {
		t.Fatal(r, err)
	}
	r = report(t, s)
	if r.Rows != 4 || len(r.Days) != 3 || r.UnbucketedCost == nil || *r.UnbucketedCost != 2 {
		t.Fatal(r)
	}
}

func TestDiagnosticsConfigOptIn(t *testing.T) {
	for _, yaml := range []string{"{}", "history: {enabled: true}", "history: {enabled: true, diagnostics: {enabled: false}}"} {
		c, err := ReadConfig("fixture", func(string) ([]byte, error) { return []byte(yaml), nil })
		if err != nil || c.History.Diagnostics.Enabled {
			t.Fatal(c, err)
		}
	}
	c, err := ReadConfig("fixture", func(string) ([]byte, error) {
		return []byte("history: {enabled: true, diagnostics: {enabled: true}}"), nil
	})
	if err != nil || !c.History.Diagnostics.Enabled {
		t.Fatal(c, err)
	}
	_, err = ReadConfig("fixture", func(string) ([]byte, error) { return []byte("history: {diagnostics: {typo: true}}"), nil })
	if err == nil {
		t.Fatal("unknown diagnostic field accepted")
	}
}
