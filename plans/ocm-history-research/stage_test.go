package research

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

var historyBounded = flag.Bool("history-bounded", false, "measure per-session and disk-stage alternatives instead of whole-source collection")

func newStage(t testing.TB) *sqlStage { // owns a generated temporary destination only
	path := filepath.Join(t.TempDir(), "stage.db")
	db := workloadDB(t, path, "rwc")
	requireSQL(t, db, stageSchema)
	return &sqlStage{db, path}
}

type sqlStage struct {
	db   *sql.DB
	path string
}

func assertUnsealed(t *testing.T, stage *sql.DB) {
	t.Helper()
	for _, table := range []string{"seal", "staged_usage"} {
		var n int
		if err := stage.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("partial stage survived: %s count=%d err=%v", table, n, err)
		}
	}
}

func TestHistoryStageSafety(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		t.Run(fmt.Sprint(v2), func(t *testing.T) {
			path, writer := fixture(t)
			if v2 {
				execute(t, writer, experimentV2Schema)
			}
			addSession(t, writer, v2, "s")
			for _, id := range []string{"a", "b", "c"} {
				insertUsage(t, writer, v2, id, "s", 1500, payload(v2, "p", "m", "", 1))
			}
			source := openReader(t, path)
			old := readFixture(t, path, extractOptions{})
			for _, failure := range []string{"row-limit", "cancel", "sink", "seal", "missing"} {
				t.Run(failure, func(t *testing.T) {
					stage := newStage(t)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					opts := extractOptions{Session: "s"}
					switch failure {
					case "row-limit":
						opts.MaxRows = 1
					case "cancel":
						opts.AfterItem = func(n int) {
							if n == 2 {
								cancel()
							}
						}
					case "sink":
						requireSQL(t, stage.db, `CREATE TRIGGER fail BEFORE INSERT ON staged_usage BEGIN SELECT RAISE(FAIL,'injected'); END`)
					case "seal":
						requireSQL(t, stage.db, `CREATE TRIGGER fail BEFORE INSERT ON seal BEGIN SELECT RAISE(FAIL,'injected'); END`)
					case "missing":
						opts.Session = "absent"
					}
					o, err := stageSession(ctx, source, stage.db, opts)
					if err == nil || o != nil {
						t.Fatal("failure returned successful stage")
					}
					assertUnsealed(t, stage.db)
					kept, state := reconcile(old, o, nil)
					if kept != old || state != "incomplete" {
						t.Fatal("failed stage replaced accepted accounting")
					}
				})
			}
			stage := newStage(t)
			o, err := stageSession(context.Background(), source, stage.db, extractOptions{Session: "s", AfterSnapshot: func() {
				table := "message"
				if v2 {
					table = "session_message"
				}
				execute(t, writer, `DELETE FROM `+table+` WHERE id='b'`)
				insertUsage(t, writer, v2, "d", "s", 1500, payload(v2, "p", "m", "", 9))
			}})
			if err != nil || o.Complete || len(o.Items) != 0 {
				t.Fatalf("stage result: %v", err)
			}
			var n int
			if err = stage.db.QueryRow(`SELECT count(*) FROM staged_usage`).Scan(&n); err != nil || n != 3 {
				t.Fatal("stage mixed snapshots", err)
			}
			var raw string
			if err = stage.db.QueryRow(`SELECT accounting FROM staged_usage WHERE key=?`, "s\x00message\x00b").Scan(&raw); err != nil {
				t.Fatal("snapshot lost deleted row", err)
			}
			if err = writeCandidate(context.Background(), stage.db, o); err == nil {
				t.Fatal("metadata-only observation published")
			}
			// Reopen the destination to prove the seal and accounting survive a restart.
			reopened := openReader(t, stage.path)
			if err = reopened.QueryRow(`SELECT count(*) FROM seal`).Scan(&n); err != nil || n != 1 {
				t.Fatal("seal not durable", err)
			}
		})
	}
}

type boundedResult struct {
	Shape, OS, Arch, GoVersion                         string
	Sessions, Staged, Deferred                         int
	TotalNS, MaxSessionNS, MaxReaderAttemptNS, StageNS int64
	StageBytes                                         int64
	AllocatedBytes, HeapIncreaseBytes                  uint64
	ExceptionalComplete                                bool
	DiagnosticStageNS                                  int64
}

func measureBounded(t *testing.T, w workload, source *sql.DB) {
	t.Helper()
	r := boundedResult{Shape: w.Name, OS: runtime.GOOS, Arch: runtime.GOARCH, GoVersion: runtime.Version()}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	combined := map[bucketKey]bucket{}
	started := time.Now()
	// Fixed generated IDs are the independent fixture inventory. This does NOT
	// measure a production inventory scan or certify source-wide completeness.
	for i := range w.Sessions {
		sid := fmt.Sprintf("s%06d", i)
		at := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		o, err := extract(ctx, source, extractOptions{Session: sid, MaxRows: 1000})
		cancel()
		elapsed := time.Since(at).Nanoseconds()
		r.MaxReaderAttemptNS = max(r.MaxReaderAttemptNS, elapsed)
		if err == nil {
			r.Sessions++
			for k, b := range aggregate(o, time.UTC, nil, nil) {
				c := combined[k]
				c.Cost += b.Cost
				c.Count += b.Count
				c.KnownCost += b.KnownCost
				c.UnknownCost += b.UnknownCost
				c.Quality = b.Quality
				for j := range c.Tokens {
					c.Tokens[j] += b.Tokens[j]
					c.UnknownTokens[j] += b.UnknownTokens[j]
				}
				combined[k] = c
			}
		} else {
			if !errors.Is(err, errBounded) && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
			r.Staged++
			stage := newStage(t)
			ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
			stageAt := time.Now()
			_, err = stageSession(ctx, source, stage.db, extractOptions{Session: sid})
			r.StageNS = time.Since(stageAt).Nanoseconds()
			cancel()
			r.MaxReaderAttemptNS = max(r.MaxReaderAttemptNS, r.StageNS)
			r.ExceptionalComplete = err == nil
			if err != nil {
				r.Deferred++
				assertUnsealed(t, stage.db)
				// Diagnostic only: determine the staging cost, NOT an allowed fallback.
				stageAt = time.Now()
				ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
				_, err = stageSession(ctx, source, stage.db, extractOptions{Session: sid})
				cancel()
				r.DiagnosticStageNS = time.Since(stageAt).Nanoseconds()
			}
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(stage.path)
			if err != nil {
				t.Fatal(err)
			}
			r.StageBytes = info.Size()
			// Check the entire day/model/token oracle after measuring memory below.
			defer verifyStage(t, w, stage.db)
		}
		r.MaxSessionNS = max(r.MaxSessionNS, time.Since(at).Nanoseconds())
		// Explicit scheduler yield after 10 sessions; fairness/renderer not measured.
		if (i+1)%10 == 0 {
			runtime.Gosched()
		}
	}
	r.TotalNS = time.Since(started).Nanoseconds()
	runtime.ReadMemStats(&after)
	r.AllocatedBytes = after.TotalAlloc - before.TotalAlloc
	r.HeapIncreaseBytes = positiveDelta(after.HeapAlloc, before.HeapAlloc)
	if r.Staged == 0 {
		if !reflect.DeepEqual(combined, workloadOracle(w)) {
			t.Fatal("session sweep differs from independent oracle")
		}
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("HISTORY_BOUNDED %s\n", encoded)
}

func verifyStage(t *testing.T, w workload, stage *sql.DB) {
	t.Helper()
	o := &Observation{Items: map[string]Usage{}}
	if !w.V2 {
		o.Quality = v1Warning
	}
	rows, err := stage.Query(`SELECT accounting FROM staged_usage`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var raw string
		var u Usage
		if err = rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal([]byte(raw), &u); err != nil {
			t.Fatal(err)
		}
		o.Items[usageKey(u)] = u
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(aggregate(o, time.UTC, nil, nil), workloadOracle(w)) {
		t.Fatal("stage differs from independent oracle")
	}
}

func TestHistoryBoundedScale(t *testing.T) {
	if !*historyBounded {
		t.Skip("opt-in bounded generated experiment")
	}
	for _, w := range scaleWorkloads {
		if *historyShape != "" && *historyShape != w.Name {
			continue
		}
		t.Run(w.Name, func(t *testing.T) {
			path := generateWorkload(t, w)
			for sample := 0; sample < *historySamples; sample++ {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHistoryScaleWorker$", "-test.timeout=55s", "-history-bounded", "-history-worker-source="+path, "-history-shape="+w.Name)
				output, err := cmd.CombinedOutput()
				cancel()
				if err != nil {
					t.Fatalf("bounded worker: %v\n%s", err, output)
				}
				found := false
				for line := range strings.SplitSeq(string(output), "\n") {
					if strings.HasPrefix(line, "HISTORY_BOUNDED ") {
						t.Log(line)
						found = true
					}
				}
				if !found {
					t.Fatal("no bounded result")
				}
			}
		})
	}
}

func TestHistoryBoundedRouting(t *testing.T) {
	path, writer := fixture(t)
	addSession(t, writer, false, "s")
	for _, id := range []string{"a", "b"} {
		insertUsage(t, writer, false, id, "s", 1500, payload(false, "p", "m", "", 1))
	}
	source := openReader(t, path)
	for _, tc := range []struct {
		name   string
		rows   int
		staged bool
	}{{"ordinary", 10, false}, {"retry", 1, true}} {
		t.Run(tc.name, func(t *testing.T) {
			stage := newStage(t)
			o, staged, err := collectSession(context.Background(), source, stage.db, "s", time.Second, time.Second, tc.rows)
			if err != nil || o == nil || staged != tc.staged || o.Complete == staged {
				t.Fatal("incorrect collection route", err)
			}
			if !staged {
				assertUnsealed(t, stage.db)
			}
		})
	}
	t.Run("cancelled", func(t *testing.T) {
		stage := newStage(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		o, _, err := collectSession(ctx, source, stage.db, "s", time.Second, time.Second, 1)
		if o != nil || err == nil {
			t.Fatal("cancelled collector returned accounting")
		}
		assertUnsealed(t, stage.db)
	})
	t.Run("metadata-limit", func(t *testing.T) {
		stage := newStage(t)
		insertUsage(t, writer, false, "large", "s", 1500, payload(false, strings.Repeat("p", 4097), "m", "", 1))
		o, err := stageSession(context.Background(), source, stage.db, extractOptions{Session: "s"})
		if o != nil || err == nil {
			t.Fatal("oversized metadata accepted")
		}
		assertUnsealed(t, stage.db)
	})
}

// A killed collector may leave a journal. Reopen its OCM-owned stage read/write
// for SQLite recovery, then reject it without a seal. No source write is needed.
func TestHistoryStageCrash(t *testing.T) {
	if sourcePath := os.Getenv("OCM_RESEARCH_CRASH_SOURCE"); sourcePath != "" {
		source := openReader(t, sourcePath)
		stage := workloadDB(t, os.Getenv("OCM_RESEARCH_CRASH_STAGE"), "rw")
		_, _ = stageSession(context.Background(), source, stage, extractOptions{Session: "s", AfterItem: func(n int) {
			if n == 2 {
				os.Exit(17)
			}
		}})
		t.Fatal("child did not exit during staging")
	}
	path, writer := fixture(t)
	addSession(t, writer, false, "s")
	for _, id := range []string{"a", "b", "c"} {
		insertUsage(t, writer, false, id, "s", 1500, payload(false, "p", "m", "", 1))
	}
	stage := newStage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHistoryStageCrash$", "-test.timeout=8s")
	cmd.Env = append(os.Environ(), "OCM_RESEARCH_CRASH_SOURCE="+path, "OCM_RESEARCH_CRASH_STAGE="+stage.path)
	output, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 17 {
		t.Fatalf("unexpected child result %v: %s", err, output)
	}
	assertUnsealed(t, stage.db)
	// Retrying the entire session succeeds; the interrupted attempt contributed nothing.
	if _, err = stageSession(context.Background(), openReader(t, path), stage.db, extractOptions{Session: "s"}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = stage.db.QueryRow(`SELECT count(*) FROM staged_usage`).Scan(&n); err != nil || n != 3 {
		t.Fatal("retry not complete", err)
	}
}
