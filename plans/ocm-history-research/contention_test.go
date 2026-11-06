package research

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/session"
	"github.com/kimoofey/tui/ocm/sources/opencode"
)

type walResult struct {
	WriterBaselineNS, WriterDuringSnapshotNS int64
	WALBytesPinned                           int64
	Busy, LogFrames, CheckpointedFrames      int
	WALBytesAfterRelease                     int64
}

func fileBytes(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func measureWAL(t *testing.T, w workload, path string, source *sql.DB) walResult {
	t.Helper()
	writer := workloadDB(t, path, "rw")
	requireSQL(t, writer, `PRAGMA wal_autocheckpoint=0`)
	table := "message"
	if w.V2 {
		table = "session_message"
	}
	update := `UPDATE ` + table + ` SET data=json_set(data,'$.cost',?) WHERE id='m000000000'`
	var result walResult
	start := time.Now()
	requireSQL(t, writer, update, .25)
	result.WriterBaselineNS = time.Since(start).Nanoseconds()
	requireSQL(t, writer, update, .125)
	requireSQL(t, writer, `PRAGMA wal_checkpoint(TRUNCATE)`)
	o, err := extract(context.Background(), source, extractOptions{AfterSnapshot: func() {
		start := time.Now()
		requireSQL(t, writer, update, .25)
		result.WriterDuringSnapshotNS = time.Since(start).Nanoseconds()
		// PASSIVE does not wait for the pinned reader. Its frame counts describe
		// checkpoint progress; busy=0 alone does not mean all frames checkpointed.
		if err := writer.QueryRow(`PRAGMA wal_checkpoint(PASSIVE)`).Scan(&result.Busy, &result.LogFrames, &result.CheckpointedFrames); err != nil {
			t.Fatal(err)
		}
		result.WALBytesPinned = fileBytes(t, path+"-wal")
	}})
	if err != nil {
		t.Fatal(err)
	}
	checkWorkload(t, w, o, nil)
	if result.WALBytesPinned == 0 || result.LogFrames <= result.CheckpointedFrames {
		t.Fatal("fixture did not demonstrate a reader pinning checkpoint progress")
	}
	// The next complete snapshot sees the changed amount, while session update
	// timestamps have not moved. This is an injected correction, not authority.
	o, err = extract(context.Background(), source, extractOptions{Session: "s000000"})
	if err != nil {
		t.Fatal(err)
	}
	key := "s000000\x00message\x00m000000000"
	if u := o.Items[key]; u.Cost == nil || *u.Cost != .25 {
		t.Fatal("new snapshot missed committed synthetic correction")
	}
	var updated int64
	sessions := "session"
	if w.V2 {
		sessions = "session_v2"
	}
	if err = writer.QueryRow(`SELECT time_updated FROM ` + sessions + ` WHERE id='s000000'`).Scan(&updated); err != nil || updated != workloadEpoch {
		t.Fatal("correction unexpectedly changed session timestamp", err)
	}
	requireSQL(t, writer, update, .125)
	var busy, log, checkpointed int
	if err = writer.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &log, &checkpointed); err != nil || busy != 0 || log != 0 {
		t.Fatal("checkpoint did not recover after reader release", err)
	}
	result.WALBytesAfterRelease = fileBytes(t, path+"-wal")
	return result
}

type browserResult struct {
	LoadToView, Navigation durationStats
	Samples                int
	ReadPhaseSamples       int
	WritePhaseSamples      int
}

func stats(values []int64) durationStats {
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return durationStats{values[0], values[len(values)/2], values[len(values)-1]}
}

func browserFrames(t *testing.T, path string, phase *atomic.Int32) browserResult {
	t.Helper()
	source, err := opencode.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = source.Close() }()
	result := browserResult{Samples: 10}
	var loads, navigation []int64
	for range result.Samples {
		if phase != nil {
			switch phase.Load() {
			case 1:
				result.ReadPhaseSamples++
			case 2:
				result.WritePhaseSamples++
			}
		}
		start := time.Now()
		listing, err := source.List(context.Background(), session.ListOptions{})
		if err != nil {
			t.Fatal(err)
		}
		model := ocm.New(ocm.Options{Sessions: listing.Sessions, TotalCount: listing.TotalCount}, 120, 40)
		frame := model.View().Content
		loads = append(loads, time.Since(start).Nanoseconds())
		if frame == "" {
			t.Fatal("headless initial frame is empty")
		}
		for i := range 4 {
			key := 'j'
			if i%2 != 0 {
				key = 'k'
			}
			start := time.Now()
			next, cmd := model.Update(tea.KeyPressMsg{Code: key})
			if cmd != nil {
				t.Fatal("navigation unexpectedly requested a command")
			}
			model = next.(ocm.Model)
			frame = model.View().Content
			navigation = append(navigation, time.Since(start).Nanoseconds())
			// With no terminal color profile the selection's color change need
			// not alter the string. Still render every updated model.
			if frame == "" {
				t.Fatal("headless navigation produced an empty frame")
			}
		}
	}
	result.LoadToView, result.Navigation = stats(loads), stats(navigation)
	return result
}

func measureBrowser(t *testing.T, path string, source *sql.DB) (browserResult, browserResult) {
	t.Helper()
	baseline := browserFrames(t, path, nil)
	destination := workloadDB(t, filepath.Join(t.TempDir(), "background.db"), "rwc")
	requireSQL(t, destination, candidateSchema)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var phase atomic.Int32
	ready := make(chan struct{})
	finished := make(chan struct{})
	var backgroundErr error
	var cancelled bool
	var once sync.Once
	go func() {
		defer close(finished)
		for {
			phase.Store(1)
			o, err := extract(ctx, source, extractOptions{AfterItem: func(int) { once.Do(func() { close(ready) }) }})
			if err == nil {
				phase.Store(2)
				err = writeCandidate(ctx, destination, o)
			}
			if err != nil {
				backgroundErr, cancelled = err, ctx.Err() != nil
				return
			}
		}
	}()
	defer func() {
		cancel()
		<-finished
		if backgroundErr != nil && !cancelled {
			t.Error("background collection failed", backgroundErr)
		}
	}()
	select {
	case <-ready:
	case <-finished:
		t.Fatal("background work failed before browser test", backgroundErr)
	}
	concurrent := browserFrames(t, path, &phase)
	return baseline, concurrent
}

func TestHistoryGeneratedContention(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		w := workload{"small-contention", 120, 4, v2}
		path := generateWorkload(t, w)
		source := openReader(t, path)
		measureWAL(t, w, path, source)
		cancelAfterItem(t, source)
		cancelBeforeItem(t, source)
		measureBrowser(t, path, source)
	}
}
