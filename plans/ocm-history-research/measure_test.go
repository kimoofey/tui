package research

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kimoofey/tui/ocm/session"
	"github.com/kimoofey/tui/ocm/sources/opencode"
)

var historyDB = flag.String("history-db", "", "explicit OpenCode database path for read-only history research; never discovered")

// Opt-in observation only: no raw identifiers/JSON/content or archive writes.
func TestHistoryExplicitPath(t *testing.T) {
	if *historyDB == "" {
		t.Skip("explicit database path not supplied; default tests use synthetic fixtures")
	}
	path, err := filepath.Abs(*historyDB)
	if err != nil {
		t.Fatal("resolve explicit path failed")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatal("explicit database must be an existing regular file")
	}
	reader := openReader(t, path)
	start := time.Now()
	o, err := extract(context.Background(), reader, extractOptions{})
	if err != nil {
		t.Fatal("extraction failed; raw error suppressed to avoid including source values")
	}
	t.Logf("source_bytes=%d sessions=%d items=%d elapsed=%s policy=%s complete=%v diagnostics=%v", info.Size(), len(o.Sessions), len(o.Items), time.Since(start), o.Policy, o.Complete, o.Diagnostics)
	// This run is not a benchmark distribution or a completeness/budget approval.
}

func BenchmarkHistoryResponsiveness(b *testing.B) {
	source := benchFixture(b, 100000, 1000)
	var seq int
	var name, path string
	if err := source.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		b.Fatal(err)
	}
	browser, err := opencode.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = browser.Close() }()
	b.Run("OneSession100Rows", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			o, err := extract(context.Background(), source, extractOptions{Session: "s000000"})
			if err != nil || len(o.Items) != 100 {
				b.Fatalf("session extraction failed: %v", err)
			}
		}
	})
	b.Run("BrowserBaseline", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := browser.List(context.Background(), session.ListOptions{}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("BrowserDuringExtraction", func(b *testing.B) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ready := make(chan struct{})
		finished := make(chan error, 1)
		var once sync.Once
		go func() {
			for {
				_, err := extract(ctx, source, extractOptions{AfterSnapshot: func() { once.Do(func() { close(ready) }) }})
				if err != nil {
					finished <- err
					return
				}
			}
		}()
		<-ready
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := browser.List(context.Background(), session.ListOptions{}); err != nil {
				b.Error(err)
				break
			}
		}
		b.StopTimer()
		cancel()
		err := <-finished
		if !errors.Is(err, context.Canceled) {
			b.Fatal(err)
		}
	})
	b.Run("CancelAfterFirstRow", func(b *testing.B) {
		var latency time.Duration
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithCancel(context.Background())
			var at time.Time
			_, err := extract(ctx, source, extractOptions{AfterItem: func(n int) {
				if n == 1 {
					at = time.Now()
					cancel()
				}
			}})
			cancel()
			if !errors.Is(err, context.Canceled) || at.IsZero() {
				b.Fatalf("cancellation failed: %v", err)
			}
			latency += time.Since(at)
		}
		b.ReportMetric(float64(latency.Nanoseconds())/float64(b.N), "cancel-ns/op")
	})
}

func TestHistorySessionScope(t *testing.T) {
	path, db := fixture(t)
	for _, id := range []string{"one", "two", "empty"} {
		addSession(t, db, false, id)
	}
	for _, id := range []string{"one", "two"} {
		insertUsage(t, db, false, id, id, 1500, payload(false, "p", "m", "", 1))
	}
	old := readFixture(t, path, extractOptions{})
	single := readFixture(t, path, extractOptions{Session: "one"})
	if len(single.Items) != 1 || len(single.Sessions) != 1 || !single.Complete {
		t.Fatal("session scope incorrect")
	}
	kept, state := reconcile(old, single, nil)
	if kept != old || state != "unresolved-scope" {
		t.Fatal("partial source scope erased other sessions")
	}
	empty := readFixture(t, path, extractOptions{Session: "empty"})
	missing := readFixture(t, path, extractOptions{Session: "missing"})
	if !empty.Complete || !missing.Complete || !empty.Sessions["empty"] || len(missing.Sessions) != 0 {
		t.Fatal("empty and missing conflated")
	}
}
