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
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

var historyUIOnly = flag.Bool("history-ui-only", false, "measure only colored UI and import under sustained pressure, skipping baseline import/refresh/correction work")

var historyGateC = flag.Bool("history-gate-c", false, "opt in to complete isolated pipeline/contention measurements")
var historyGateWorker = flag.Bool("history-gate-worker", false, "internal Gate C worker; requires a generated fixture marker")

func optionalBytes(path string) int64 {
	info, e := os.Stat(path)
	if e != nil {
		return 0
	}
	return info.Size()
}

type resourceResult struct {
	PeakCombinedDiskBytes                                                                            int64
	SampledHeapIncrease, SampledHeapInuseIncrease                                                    uint64
	ArchiveBytes, ArchiveWALBytes, ArchiveJournalBytes, StageBytes, StageJournalBytes, StageWALBytes int64
}
type resourceSampler struct {
	mu     sync.Mutex
	stages []string
	stop   chan struct{}
	done   chan struct{}
	result resourceResult
}

func sampleResources(archive string) *resourceSampler {
	s := &resourceSampler{stop: make(chan struct{}), done: make(chan struct{})}
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				s.result.SampledHeapIncrease = max(s.result.SampledHeapIncrease, positiveDelta(m.HeapAlloc, base.HeapAlloc))
				s.result.SampledHeapInuseIncrease = max(s.result.SampledHeapInuseIncrease, positiveDelta(m.HeapInuse, base.HeapInuse))
				s.result.ArchiveBytes = max(s.result.ArchiveBytes, optionalBytes(archive))
				s.result.ArchiveWALBytes = max(s.result.ArchiveWALBytes, optionalBytes(archive+"-wal"))
				s.result.ArchiveJournalBytes = max(s.result.ArchiveJournalBytes, optionalBytes(archive+"-journal"))
				disk := optionalBytes(archive) + optionalBytes(archive+"-wal") + optionalBytes(archive+"-journal")
				s.mu.Lock()
				for _, path := range s.stages {
					disk += optionalBytes(path) + optionalBytes(path+"-wal") + optionalBytes(path+"-journal")
					s.result.StageBytes = max(s.result.StageBytes, optionalBytes(path))
					s.result.StageJournalBytes = max(s.result.StageJournalBytes, optionalBytes(path+"-journal"))
					s.result.StageWALBytes = max(s.result.StageWALBytes, optionalBytes(path+"-wal"))
				}
				s.mu.Unlock()
				s.result.PeakCombinedDiskBytes = max(s.result.PeakCombinedDiskBytes, disk)
			}
		}
	}()
	return s
}
func (s *resourceSampler) add(path string) {
	s.mu.Lock()
	s.stages = append(s.stages, path)
	s.mu.Unlock()
}
func (s *resourceSampler) finish() resourceResult { close(s.stop); <-s.done; return s.result }

type scenarioResult struct {
	Name, Status                  string
	CaptureNS, PublishNS, TotalNS int64
}
type gateResult struct {
	Shape, OS, Arch, GoVersion, SQLite string
	Sample                             int
	Initial, Unchanged, PressureSweep  pipelineTiming
	Matrix                             []scenarioResult
	BrowserBaseline, BrowserConcurrent loopResult
	WriterBaseline, WriterConcurrent   pressureResult
	Cancellation                       []cancellationResult
	Resources                          resourceResult
	SavedQueryNS, WorkerPeakRSSBytes   int64
}

func refreshMatrix(t *testing.T, w workload, path string, source, archive *sql.DB) []scenarioResult {
	writer := workloadDB(t, path, "rw")
	table, sessions := "message", "session"
	if w.V2 {
		table, sessions = "session_message", "session_v2"
	}
	var original string
	if err := writer.QueryRow(`SELECT data FROM ` + table + ` WHERE id='m000000001'`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	defer func() {
		requireSQL(t, writer, `UPDATE `+table+` SET data=? WHERE id='m000000001'`, original)
		requireSQL(t, writer, `DELETE FROM `+table+` WHERE session_id='research-new'`)
		requireSQL(t, writer, `DELETE FROM `+sessions+` WHERE id='research-new'`)
	}()
	var results []scenarioResult
	run := func(name, sid string, authority map[string]bool, want string) {
		at := time.Now()
		o, err := extract(context.Background(), source, extractOptions{Session: sid, MaxRows: 1000})
		var stream *sql.DB
		if errors.Is(err, errBounded) {
			stage := newStage(t)
			stream = stage.db
			defer func() { _ = stream.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			o, err = stageSession(ctx, source, stream, extractOptions{Session: sid})
			cancel()
		}
		capture := time.Since(at).Nanoseconds()
		if err != nil {
			t.Fatal(err)
		}
		var gen int64
		err = archive.QueryRow(`SELECT generation FROM accepted WHERE session=?`, sid).Scan(&gen)
		if err != nil && err != sql.ErrNoRows {
			t.Fatal(err)
		}
		pub := time.Now()
		status, err := publishSession(context.Background(), archive, sid, o, stream, publishOptions{Expected: gen, Corrections: authority})
		if err != nil || status != want {
			t.Fatal(name, status, want, err)
		}
		results = append(results, scenarioResult{name, status, capture, time.Since(pub).Nanoseconds(), time.Since(at).Nanoseconds()})
	}
	if w.V2 {
		requireSQL(t, writer, `INSERT INTO session_v2 VALUES('research-new','Synthetic','/synthetic',1,1,0,NULL,NULL)`)
	} else {
		requireSQL(t, writer, `INSERT INTO session VALUES('research-new','Synthetic','/synthetic',1,1,0,NULL)`)
	}
	// New session has exactly 100 eligible rows, independent of the huge shape.
	for i := range 100 {
		data := workloadJSON(w, i, 128, .125)
		id := fmt.Sprintf("research-%03d", i)
		if w.V2 {
			requireSQL(t, writer, `INSERT INTO session_message VALUES(?,'research-new','assistant',?,1,1,?)`, id, i, data)
		} else {
			requireSQL(t, writer, `INSERT INTO message VALUES(?,'research-new',1,1,?)`, id, data)
		}
	}
	run("new-100-row", "research-new", nil, "accepted")
	run("unchanged-100-row", "research-new", nil, "unchanged")
	data := workloadJSON(w, 0, 128, .125)
	if w.V2 {
		requireSQL(t, writer, `INSERT INTO session_message VALUES('research-appended','research-new','assistant',101,1,1,?)`, data)
	} else {
		requireSQL(t, writer, `INSERT INTO message VALUES('research-appended','research-new',1,1,?)`, data)
	}
	run("appended-usage", "research-new", nil, "accepted")
	requireSQL(t, writer, `UPDATE `+table+` SET data=json_set(data,'$.cost',0) WHERE id='research-000'`)
	before := acceptedCost(t, archive)
	run("changed-without-authority", "research-new", nil, "unresolved-correction")
	if acceptedCost(t, archive) != before {
		t.Fatal("unresolved correction changed totals")
	}
	run("changed-with-authority", "research-new", map[string]bool{"research-new\x00message\x00research-000": true}, "accepted")
	if acceptedCost(t, archive) != before-.125 {
		t.Fatal("authorized correction did not apply")
	}
	// Native old usage correction, with session timestamp left unchanged. For the
	// huge shape this also exercises staged correction reconciliation/publication.
	requireSQL(t, writer, `UPDATE `+table+` SET data=json_set(data,'$.cost',0) WHERE id='m000000001'`)
	sid := fmt.Sprintf("s%06d", 1%w.Sessions)
	run("old-native-correction-without-authority", sid, nil, "unresolved-correction")
	run("old-native-correction-with-authority", sid, map[string]bool{sid + "\x00message\x00m000000001": true}, "accepted")
	var stamp int64
	if err := writer.QueryRow(`SELECT time_updated FROM `+sessions+` WHERE id=?`, sid).Scan(&stamp); err != nil || stamp != workloadEpoch {
		t.Fatal("session timestamp changed", err)
	}
	return results
}

func TestHistoryGateC(t *testing.T) {
	if !*historyGateC {
		t.Skip("opt-in complete pipeline experiment")
	}
	if *historySamples < 1 || *historySamples > 20 {
		t.Fatal("history-samples must be 1-20")
	}
	matched := false
	for _, w := range scaleWorkloads {
		if *historyShape != "" && *historyShape != w.Name {
			continue
		}
		matched = true
		t.Run(w.Name, func(t *testing.T) {
			path := generateWorkload(t, w)
			for sample := 1; sample <= *historySamples; sample++ {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHistoryGateCWorker$", "-test.v", "-test.timeout=110s", "-history-gate-worker", "-history-worker-source="+path, "-history-shape="+w.Name)
				if *historyUIOnly {
					cmd.Args = append(cmd.Args, "-history-ui-only")
				}
				output, err := cmd.CombinedOutput()
				cancel()
				if err != nil {
					t.Fatalf("Gate C worker failed: %v\n%s", err, output)
				}
				found := false
				for line := range strings.SplitSeq(string(output), "\n") {
					if data, ok := strings.CutPrefix(line, "HISTORY_GATE_C "); ok {
						var r gateResult
						if err = json.Unmarshal([]byte(data), &r); err != nil {
							t.Fatal(err)
						}
						r.Sample = sample
						usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
						if !ok {
							t.Fatal("worker rusage unavailable")
						}
						r.WorkerPeakRSSBytes = usage.Maxrss
						if runtime.GOOS == "linux" {
							r.WorkerPeakRSSBytes *= 1024
						}
						raw, _ := json.Marshal(r)
						t.Logf("HISTORY_GATE_C %s", raw)
						found = true
					}
				}
				if !found {
					t.Fatal("missing Gate C result")
				}
			}
		})
	}
	if !matched {
		t.Fatal("unknown history-shape")
	}
}
func TestHistoryGateCWorker(t *testing.T) {
	if !*historyGateWorker {
		t.Skip("internal Gate C generated worker")
	}
	w := findWorkload(t)
	path := *historyWorkerSource
	source := openReader(t, path)
	var marker string
	if err := source.QueryRow(`SELECT name FROM history_workload`).Scan(&marker); err != nil || marker != w.Name {
		t.Fatal("generated fixture marker mismatch", err)
	}
	r := gateResult{Shape: w.Name, OS: runtime.GOOS, Arch: runtime.GOARCH, GoVersion: runtime.Version()}
	if err := source.QueryRow(`SELECT sqlite_version()`).Scan(&r.SQLite); err != nil {
		t.Fatal(err)
	}
	archive, archivePath := pipelineArchive(t)
	sampler := sampleResources(archivePath)
	defer func() {
		if sampler != nil {
			_ = sampler.finish()
		}
	}()
	hooks := pipelineHooks{StageFile: sampler.add}
	var err error
	if !*historyUIOnly {
		r.Initial, err = sweep(context.Background(), source, archive, stageFactory(t), hooks)
		if err != nil {
			t.Fatal(err)
		}
		if r.Initial.Deferred == 0 {
			assertPipelineOracle(t, w, archive)
		}
		at := time.Now()
		var count int
		if err = archive.QueryRow(`SELECT coalesce(sum(steps),0) FROM session_daily`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		r.SavedQueryNS = time.Since(at).Nanoseconds()
		r.Unchanged, err = sweep(context.Background(), source, archive, stageFactory(t), hooks)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Writer baseline, then actual program/input/renderer baseline.
	baseline := startPressure(path, w.V2)
	time.Sleep(250 * time.Millisecond)
	r.WriterBaseline, err = baseline.stop()
	if err != nil {
		t.Fatal(err)
	}
	baselineStop := make(chan struct{})
	time.AfterFunc(350*time.Millisecond, func() { close(baselineStop) })
	r.BrowserBaseline, err = runInputLoop(t, path, nil, nil, baselineStop)
	if err != nil {
		t.Fatal(err)
	}
	// Pressure run measures a fresh import into the same private archive.
	requireSQL(t, archive, `DELETE FROM accepted; DELETE FROM contributions; DELETE FROM session_daily; PRAGMA wal_checkpoint(TRUNCATE)`)
	var phase atomic.Int32
	hooks.Phase = func(s string) {
		if s == "publication" {
			phase.Store(2)
		} else {
			phase.Store(1)
		}
	}
	pressure := startPressure(path, w.V2)
	stop := make(chan struct{})
	ready := make(chan struct{})
	loopDone := make(chan struct{})
	var loopErr error
	go func() {
		defer close(loopDone)
		r.BrowserConcurrent, loopErr = runInputLoop(t, path, &phase, ready, stop)
	}()
	select {
	case <-ready:
	case <-loopDone:
		t.Fatal("program exited before pressure sweep", loopErr)
	case <-time.After(5 * time.Second):
		t.Fatal("program readiness timeout")
	}
	r.PressureSweep, err = sweep(context.Background(), source, archive, stageFactory(t), hooks)
	close(stop)
	<-loopDone
	r.WriterConcurrent, loopErr = finishPressure(pressure, loopErr)
	if err != nil || loopErr != nil {
		t.Fatal(err, loopErr)
	}
	if r.PressureSweep.Deferred == 0 {
		assertPipelineOracle(t, w, archive)
	}
	if r.WriterConcurrent.Writes == 0 || r.WriterConcurrent.Checkpoints == 0 {
		t.Fatal("sustained writer/checkpoint pressure not observed")
	}
	r.Resources = sampler.finish()
	sampler = nil
	if !*historyUIOnly {
		r.Matrix = refreshMatrix(t, w, path, source, archive)
		r.Cancellation = measureBlocked(t)
	}
	writer := workloadDB(t, path, "rw")
	var busy, log, copied int
	if err = writer.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &log, &copied); err != nil || busy != 0 || log != 0 {
		t.Fatal("source WAL did not recover", busy, log, copied, err)
	}
	r.WriterConcurrent.FinalWALBytes = optionalBytes(path + "-wal")
	raw, _ := json.Marshal(r)
	fmt.Printf("HISTORY_GATE_C %s\n", raw)
}
func finishPressure(p *pressureWorker, prior error) (pressureResult, error) {
	r, err := p.stop()
	if prior != nil {
		return r, prior
	}
	return r, err
}

// Ensure artifacts remain temporary and no repository archive can be selected.
func TestHistoryPipelineDefaultSweep(t *testing.T) {
	w := workload{"pipeline-default", 120, 4, true}
	path := generateWorkload(t, w)
	source := openReader(t, path)
	archive, archivePath := pipelineArchive(t)
	if filepath.Base(archivePath) != "archive.db" {
		t.Fatal(archivePath)
	}
	r, err := sweep(context.Background(), source, archive, stageFactory(t), pipelineHooks{})
	if err != nil || r.Sessions != 4 {
		t.Fatal(r, err)
	}
	assertPipelineOracle(t, w, archive)
}
