package research

// Opt-in generated-fixture profiling. Profiles stay in an explicit private
// directory; ordinary tests do not create profiles or discover native sources.
import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"testing"
	"time"
)

var historyProfileDir = flag.String("history-profile-dir", "", "explicit output directory for synthetic stage CPU profiles")

func TestHistoryStageProfile(t *testing.T) {
	if *historyProfileDir == "" {
		t.Skip("opt-in synthetic stage profiling")
	}
	if err := os.MkdirAll(*historyProfileDir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, w := range scaleWorkloads {
		if w.Sessions != 1 || (*historyShape != "" && *historyShape != w.Name) {
			continue
		}
		path := generateWorkload(t, w)
		cmd := exec.Command(os.Args[0], "-test.run=^TestHistoryStageProfileWorker$", "-test.v", "-history-worker-source="+path, "-history-shape="+w.Name, "-history-profile-dir="+*historyProfileDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("profile worker %v: %s", err, output)
		}
		t.Log(string(output))
	}
}

func TestHistoryStageProfileWorker(t *testing.T) {
	if *historyWorkerSource == "" || *historyProfileDir == "" {
		t.Skip("internal generated profile worker")
	}
	w := findWorkload(t)
	source := openReader(t, *historyWorkerSource)
	var marker string
	if err := source.QueryRow(`SELECT name FROM history_workload`).Scan(&marker); err != nil || marker != w.Name {
		t.Fatal("requires generated fixture")
	}
	stage := newStage(t)
	file, err := os.Create(filepath.Join(*historyProfileDir, w.Name+".cpu"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if err = pprof.StartCPUProfile(file); err != nil {
		t.Fatal(err)
	}
	var snapshotAt, firstAt time.Time
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	_, err = stageSession(ctx, source, stage.db, extractOptions{Session: "s000000", AfterSnapshot: func() { snapshotAt = time.Now() }, AfterItem: func(n int) {
		if n == 1 {
			firstAt = time.Now()
		}
	}})
	elapsed := time.Since(started)
	cancel()
	pprof.StopCPUProfile()
	if err != nil {
		t.Fatal(err)
	}
	verifyStage(t, w, stage.db)
	query, args := usageQuery(map[bool]string{false: "session", true: "session_v2"}[w.V2], "s000000")
	rows, err := source.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err = rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		t.Log("PLAN", detail)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	_ = rows.Close()

	data, _ := json.Marshal(map[string]any{"shape": w.Name, "stage_ns": elapsed.Nanoseconds(), "snapshot_to_first_row_ns": firstAt.Sub(snapshotAt).Nanoseconds(), "profiled": true})
	fmt.Printf("HISTORY_PROFILE %s\n", data)
}
