package research

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

var historyScale = flag.Bool("history-scale", false, "opt in to generated V1/V2 scale measurements in fresh processes")
var historyShape = flag.String("history-shape", "", "optional exact scale shape (v1/v2-10k, v1/v2-100k, v1/v2-huge)")
var historySamples = flag.Int("history-samples", 5, "fresh measurement processes per shape (1-20)")
var historyWorkerSource = flag.String("history-worker-source", "", "internal worker: explicit generated fixture, validated by its marker")

type durationStats struct{ MinNS, MedianNS, MaxNS int64 }
type scaleResult struct {
	Shape, GoVersion, OS, Arch, SQLite string
	Sample                             int
	SourceBytes, JSONBytes, MaxJSON    int64
	Items, Sessions                    int
	ExtractNS, ImportNS, UnchangedNS   int64
	SessionNS, SavedQueryNS            int64
	AllocatedBytes                     uint64
	SampledHeapIncreaseBytes           uint64
	RetainedObservationHeapBytes       uint64
	WorkerPeakRSSBytes                 int64
	ArchiveBytes, ArchiveWALBytes      int64
	CancelAfterItemNS                  int64
	CancelBeforeItemNS                 int64
	CancelBeforeItemObserved           bool
	WAL                                walResult
	BrowserBaseline, BrowserConcurrent browserResult
}

func findWorkload(t *testing.T) workload {
	t.Helper()
	for _, w := range scaleWorkloads {
		if w.Name == *historyShape {
			return w
		}
	}
	t.Fatal("unknown history-shape")
	return workload{}
}

// Parent owns synthetic generation; the measured child never allocates fixtures.
// Default tests skip both this runner and its worker. No source path discovery.
func TestHistoryScale(t *testing.T) {
	if !*historyScale {
		t.Skip("opt-in generated scale experiment; use -args -history-scale")
	}
	if *historySamples < 1 || *historySamples > 20 {
		t.Fatal("history-samples must be 1-20")
	}
	matched := false
	for _, w := range scaleWorkloads {
		if *historyShape != "" && w.Name != *historyShape {
			continue
		}
		matched = true
		t.Run(w.Name, func(t *testing.T) {
			path := generateWorkload(t, w)
			for sample := 1; sample <= *historySamples; sample++ {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHistoryScaleWorker$", "-test.v", "-test.timeout=110s", "-history-worker-source="+path, "-history-shape="+w.Name)
				output, err := cmd.CombinedOutput()
				cancel()
				if err != nil {
					t.Fatalf("scale worker failed: %v\n%s", err, output)
				}
				var result scaleResult
				found := false
				for line := range strings.SplitSeq(string(output), "\n") {
					if data, ok := strings.CutPrefix(line, "HISTORY_WORKER "); ok {
						if err = json.Unmarshal([]byte(data), &result); err != nil {
							t.Fatal(err)
						}
						found = true
					}
				}
				if !found {
					t.Fatal("worker did not return a result")
				}
				usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
				if !ok {
					t.Fatal("worker resource usage unavailable")
				}
				result.WorkerPeakRSSBytes = usage.Maxrss
				if runtime.GOOS == "linux" {
					result.WorkerPeakRSSBytes *= 1024
				}
				result.Sample = sample
				encoded, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("HISTORY_SCALE %s", encoded)
			}
		})
	}
	if !matched {
		t.Fatal("unknown history-shape")
	}
}

func TestHistoryScaleWorker(t *testing.T) {
	if *historyWorkerSource == "" {
		t.Skip("internal generated-data worker")
	}
	w := findWorkload(t)
	source := openReader(t, *historyWorkerSource)
	var marker string
	if err := source.QueryRow(`SELECT name FROM history_workload`).Scan(&marker); err != nil || marker != w.Name {
		t.Fatal("worker requires its generated fixture marker")
	}
	result := scaleResult{Shape: w.Name, GoVersion: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, Sessions: w.Sessions}
	if err := source.QueryRow(`SELECT sqlite_version()`).Scan(&result.SQLite); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(*historyWorkerSource)
	if err != nil {
		t.Fatal(err)
	}
	result.SourceBytes = info.Size()
	table := "message"
	if w.V2 {
		table = "session_message"
	}
	if err = source.QueryRow(`SELECT sum(length(CAST(data AS BLOB))),max(length(CAST(data AS BLOB))) FROM `+table).Scan(&result.JSONBytes, &result.MaxJSON); err != nil {
		t.Fatal(err)
	}
	destinationPath := filepath.Join(t.TempDir(), "candidate.db")
	destination := workloadDB(t, destinationPath, "rwc")
	requireSQL(t, destination, candidateSchema)
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var peak atomic.Uint64
	peak.Store(before.HeapAlloc)
	stop, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				peak.Store(max(peak.Load(), m.HeapAlloc))
			}
		}
	}()
	start := time.Now()
	o, err := extract(context.Background(), source, extractOptions{})
	result.ExtractNS = time.Since(start).Nanoseconds()
	if err == nil {
		err = writeCandidate(context.Background(), destination, o) // includes grouping
	}
	result.ImportNS = time.Since(start).Nanoseconds()
	close(stop)
	<-stopped
	if err != nil {
		t.Fatal(err)
	}
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	result.AllocatedBytes = after.TotalAlloc - before.TotalAlloc
	result.SampledHeapIncreaseBytes = positiveDelta(max(peak.Load(), after.HeapAlloc), before.HeapAlloc)
	runtime.GC()
	runtime.ReadMemStats(&after)
	result.RetainedObservationHeapBytes = positiveDelta(after.HeapAlloc, before.HeapAlloc)
	checkWorkload(t, w, o, destination)
	result.Items = len(o.Items)
	runtime.KeepAlive(o)
	o = nil

	start = time.Now()
	var steps int
	if err = destination.QueryRow(`SELECT sum(steps) FROM daily`).Scan(&steps); err != nil || steps != result.Items {
		t.Fatal("saved aggregate query failed", err)
	}
	result.SavedQueryNS = time.Since(start).Nanoseconds()
	info, err = os.Stat(destinationPath)
	if err != nil {
		t.Fatal(err)
	}
	result.ArchiveBytes = info.Size()
	if wal, e := os.Stat(destinationPath + "-wal"); e == nil {
		result.ArchiveWALBytes = wal.Size()
	}

	// This is the actual prototype's unchanged path: a full read and rewrite,
	// not a timestamp-based incremental collector or an approved refresh policy.
	start = time.Now()
	o, err = extract(context.Background(), source, extractOptions{})
	if err == nil {
		err = writeCandidate(context.Background(), destination, o)
	}
	result.UnchangedNS = time.Since(start).Nanoseconds()
	if err != nil {
		t.Fatal(err)
	}
	checkWorkload(t, w, o, destination)
	o = nil
	runtime.GC()
	start = time.Now()
	o, err = extract(context.Background(), source, extractOptions{Session: "s000000"})
	result.SessionNS = time.Since(start).Nanoseconds()
	if err != nil || !o.Complete || len(o.Sessions) != 1 {
		t.Fatal("session extraction failed", err)
	}
	o = nil
	runtime.GC()
	result.CancelAfterItemNS = cancelAfterItem(t, source)
	result.CancelBeforeItemNS, result.CancelBeforeItemObserved = cancelBeforeItem(t, source)
	result.WAL = measureWAL(t, w, *historyWorkerSource, source)
	result.BrowserBaseline, result.BrowserConcurrent = measureBrowser(t, *historyWorkerSource, source)
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("HISTORY_WORKER %s\n", encoded)
}

func positiveDelta(after, before uint64) uint64 {
	if after > before {
		return after - before
	}
	return 0
}

func cancelAfterItem(t *testing.T, source *sql.DB) int64 {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var at time.Time
	o, err := extract(ctx, source, extractOptions{AfterItem: func(n int) {
		if n == 1 {
			at = time.Now()
			cancel()
		}
	}})
	latency := time.Since(at)
	if at.IsZero() || ctx.Err() == nil || err == nil || o != nil {
		t.Fatal("cancelled extraction returned a publishable result")
	}
	return latency.Nanoseconds()
}

func cancelBeforeItem(t *testing.T, source *sql.DB) (int64, bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requested := make(chan time.Time, 1)
	var timer *time.Timer
	items := 0
	o, err := extract(ctx, source, extractOptions{AfterSnapshot: func() {
		// Timed experiment, not a deterministic claim about SQLite's internal
		// execution point. Record whether any row reached Go before cancellation.
		timer = time.AfterFunc(time.Millisecond, func() {
			at := time.Now()
			cancel()
			requested <- at
		})
	}, AfterItem: func(int) { items++ }})
	returned := time.Now()
	if timer == nil {
		t.Fatal("snapshot did not open")
	}
	if timer.Stop() {
		return 0, false // extraction finished before the cancellation request
	}
	at := <-requested
	if returned.Before(at) {
		return 0, false
	}
	latency := returned.Sub(at)
	if ctx.Err() == nil || err == nil || o != nil {
		t.Fatal("cancelled extraction returned a publishable result")
	}
	return latency.Nanoseconds(), items == 0
}
