package research

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/session"
	"github.com/kimoofey/tui/ocm/sources/opencode"
)

// The wrapper adds a different full marker line after each real decoded key.
// The renderer remains enabled. Times end when the output sink receives that
// marker; this includes decoder/event loop/frame pacing, not terminal painting.
type frameModel struct {
	inner    tea.Model
	sequence int
}

func (m frameModel) Init() tea.Cmd { return m.inner.Init() }
func (m frameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && (k.Code == 'j' || k.Code == 'k') {
		m.sequence++
	}
	var cmd tea.Cmd
	m.inner, cmd = m.inner.Update(msg)
	return m, cmd
}
func (m frameModel) View() tea.View {
	v := m.inner.View()
	first := strings.LastIndexByte(v.Content, '\n')
	if first >= 0 {
		v.SetContent(v.Content[:first+1] + frameMarker(m.sequence))
	} else {
		v.SetContent(frameMarker(m.sequence))
	}
	return v
}
func frameMarker(n int) string { return strings.Repeat(string(rune('A'+n%26)), 48) }

type renderedFrame struct {
	sequence int
	at       time.Time
}
type frameSink struct {
	sawColor bool
	mu       sync.Mutex
	pending  []byte
	seen     int
	frames   chan renderedFrame
}

func (s *frameSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = append(s.pending, p...)
	s.sawColor = s.sawColor || bytes.Contains(s.pending, []byte("38;2;")) || bytes.Contains(s.pending, []byte("48;2;"))
	for {
		marker := []byte(frameMarker(s.seen + 1))
		index := bytes.Index(s.pending, marker)
		if index < 0 {
			break
		}
		s.pending = s.pending[index+len(marker):]
		s.seen++
		s.frames <- renderedFrame{s.seen, time.Now()}
	}
	if len(s.pending) > 65536 {
		s.pending = append([]byte(nil), s.pending[len(s.pending)-32768:]...)
	}
	return len(p), nil
}

type loopResult struct {
	ColorProfile                                string
	ColorOutput                                 bool
	Samples, PublicationStarts, PublicationEnds int
	InputToOutput                               durationStats
	LoadToProgramOutputNS                       int64
}

// runInputLoop drives actual bytes through Bubble Tea's input reader; it never
// invokes Update/View directly and never launches a terminal application.
func runInputLoop(t *testing.T, path string, phase *atomic.Int32, ready chan<- struct{}, stop <-chan struct{}) (r loopResult, err error) {
	r.ColorProfile = "truecolor"
	load := time.Now()
	source, e := opencode.Open(path)
	if e != nil {
		return r, e
	}
	defer func() { _ = source.Close() }()
	listing, e := source.List(context.Background(), session.ListOptions{})
	if e != nil {
		return r, e
	}
	in, out := io.Pipe()
	sink := &frameSink{seen: 0, frames: make(chan renderedFrame, 256)}
	// Sequence zero is startup; only input-derived sequence markers are timed.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p := tea.NewProgram(frameModel{inner: ocm.New(ocm.Options{Sessions: listing.Sessions, TotalCount: listing.TotalCount}, 120, 40)}, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(sink), tea.WithWindowSize(120, 40), tea.WithColorProfile(colorprofile.TrueColor), tea.WithEnvironment([]string{"TERM=xterm-256color"}), tea.WithoutSignalHandler())
	done := make(chan error, 1)
	go func() { _, e := p.Run(); done <- e }()
	defer func() {
		_ = out.Close()
		p.Quit()
		select {
		case e := <-done:
			if err == nil && e != nil {
				err = e
			}
		case <-time.After(3 * time.Second):
			p.Kill()
			err = errors.New("renderer shutdown timeout")
		}
		_ = in.Close()
	}()
	var samples []int64
	for n := 1; ; n++ {
		if n > 1 {
			select {
			case <-stop:
				r.Samples = len(samples)
				r.InputToOutput = stats(samples)
				sink.mu.Lock()
				r.ColorOutput = sink.sawColor
				sink.mu.Unlock()
				if !r.ColorOutput {
					return r, errors.New("TrueColor renderer did not emit RGB styles")
				}
				return r, nil
			default:
			}
		}
		if phase != nil && phase.Load() == 2 {
			r.PublicationStarts++
		}
		at := time.Now()
		key := byte('j')
		if n%2 == 0 {
			key = 'k'
		}
		if _, e = out.Write([]byte{key}); e != nil {
			return r, e
		}
		select {
		case frame := <-sink.frames:
			if frame.sequence != n {
				return r, fmt.Errorf("frame sequence %d != %d", frame.sequence, n)
			}
			samples = append(samples, frame.at.Sub(at).Nanoseconds())
			if phase != nil && phase.Load() == 2 {
				r.PublicationEnds++
			}
			if n == 1 {
				r.LoadToProgramOutputNS = time.Since(load).Nanoseconds()
				if ready != nil {
					close(ready)
				}
			}
		case e = <-done:
			done <- e
			return r, fmt.Errorf("program stopped before frame: %v", e)
		case <-ctx.Done():
			return r, errors.New("input-to-output marker timeout")
		}
	}
}

type pressureResult struct {
	Writes, Checkpoints                              int
	Writer                                           durationStats
	MaxWALBytes                                      int64
	LastBusy, LastLog, LastCopied                    int
	MaxUncheckpointedFrames, PinnedCheckpointSamples int
	FinalWALBytes                                    int64
}
type pressureWorker struct {
	cancel context.CancelFunc
	done   chan struct{}
	result pressureResult
	err    error
}

func startPressure(path string, v2 bool) *pressureWorker {
	ctx, cancel := context.WithCancel(context.Background())
	p := &pressureWorker{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(p.done)
		db, e := sql.Open("sqlite", "file:"+path+"?mode=rw")
		if e != nil {
			p.err = e
			return
		}
		defer func() { _ = db.Close() }()
		db.SetMaxOpenConns(1)
		if _, e = db.Exec(`PRAGMA wal_autocheckpoint=0; PRAGMA busy_timeout=10`); e != nil {
			p.err = e
			return
		}
		table := "message"
		if v2 {
			table = "session_message"
		}
		var timings []int64
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				if len(timings) > 0 {
					p.result.Writer = stats(timings)
				}
				return
			case <-ticker.C:
				at := time.Now()
				_, e = db.ExecContext(ctx, `UPDATE `+table+` SET data=json_set(data,'$.researchPressure',?) WHERE id='m000000001'`, p.result.Writes%2)
				if e != nil {
					if ctx.Err() == nil {
						p.err = e
					}
					if len(timings) > 0 {
						p.result.Writer = stats(timings)
					}
					return
				}
				timings = append(timings, time.Since(at).Nanoseconds())
				p.result.Writes++
				if p.result.Writes%10 == 0 {
					e = db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`).Scan(&p.result.LastBusy, &p.result.LastLog, &p.result.LastCopied)
					if e != nil {
						if ctx.Err() == nil {
							p.err = e
						}
						return
					}
					p.result.Checkpoints++
					p.result.MaxUncheckpointedFrames = max(p.result.MaxUncheckpointedFrames, p.result.LastLog-p.result.LastCopied)
					if p.result.LastLog > p.result.LastCopied {
						p.result.PinnedCheckpointSamples++
					}
				}
				p.result.MaxWALBytes = max(p.result.MaxWALBytes, optionalBytes(path+"-wal"))
			}
		}
	}()
	return p
}
func (p *pressureWorker) stop() (pressureResult, error) { p.cancel(); <-p.done; return p.result, p.err }

type cancellationResult struct {
	Case            string
	ReturnNS        int64
	Error           string
	Recovered       bool
	PendingAtCancel bool
}

func measureBlocked(t *testing.T) []cancellationResult {
	t.Helper()
	var results []cancellationResult
	for _, kind := range []string{"source-exclusive-lock", "archive-write-lock", "connection-pool-wait", "coordinator-wait"} {
		path, holder := fixture(t)
		waiter := openReader(t, path)
		if kind == "archive-write-lock" {
			waiter = workloadDB(t, path, "rw")
		}
		requireSQL(t, waiter, `PRAGMA busy_timeout=10`)
		var release func()
		var lockWaiter *os.File
		switch kind {
		case "source-exclusive-lock":
			requireSQL(t, holder, `PRAGMA journal_mode=DELETE; BEGIN EXCLUSIVE`)
			release = func() { requireSQL(t, holder, `ROLLBACK`) }
		case "archive-write-lock":
			requireSQL(t, holder, `BEGIN IMMEDIATE`)
			release = func() { requireSQL(t, holder, `ROLLBACK`) }
		case "connection-pool-wait":
			conn, e := waiter.Conn(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			release = func() { _ = conn.Close() }
		default:
			lockHolder, e := os.CreateTemp(t.TempDir(), "coordinator-lock")
			if e != nil {
				t.Fatal(e)
			}
			lockWaiter, e = os.OpenFile(lockHolder.Name(), os.O_RDWR, 0600)
			if e != nil {
				t.Fatal(e)
			}
			if e = syscall.Flock(int(lockHolder.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
				t.Fatal(e)
			}
			release = func() {
				_ = syscall.Flock(int(lockHolder.Fd()), syscall.LOCK_UN)
				_ = lockHolder.Close()
				_ = lockWaiter.Close()
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		started := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			close(started)
			switch kind {
			case "coordinator-wait":
				for {
					e := syscall.Flock(int(lockWaiter.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
					if e == nil {
						_ = syscall.Flock(int(lockWaiter.Fd()), syscall.LOCK_UN)
						done <- errors.New("unexpected lock admission")
						return
					}
					if !errors.Is(e, syscall.EWOULDBLOCK) {
						done <- e
						return
					}
					select {
					case <-ctx.Done():
						done <- ctx.Err()
						return
					case <-time.After(time.Millisecond):
					}
				}
			case "archive-write-lock":
				_, e := waiter.ExecContext(ctx, `INSERT INTO session VALUES('x','Synthetic','/synthetic',1,1,0,NULL)`)
				done <- e
			default:
				var n int
				e := waiter.QueryRowContext(ctx, `SELECT count(*) FROM session`).Scan(&n)
				done <- e
			}
		}()
		<-started
		// Experimental dwell establishes an opportunity to block, not proof of a
		// particular SQLite internal point. Early SQLITE_BUSY is recorded separately.
		timer := time.NewTimer(2 * time.Millisecond)
		<-timer.C
		pending := true
		var early error
		select {
		case early = <-done:
			pending = false
		default:
		}
		at := time.Now()
		cancel()
		var e error
		if pending {
			e = <-done
		} else {
			e = early
		}
		elapsed := time.Since(at).Nanoseconds()
		release()
		r := cancellationResult{Case: kind, ReturnNS: elapsed, PendingAtCancel: pending}
		if e != nil {
			r.Error = e.Error()
		}
		var n int
		if e = waiter.QueryRow(`SELECT count(*) FROM session`).Scan(&n); e == nil {
			r.Recovered = true
		}
		if !r.Recovered {
			t.Fatal("blocked connection did not recover", kind, e)
		}
		results = append(results, r)
	}
	return results
}

func TestHistoryProgramLoop(t *testing.T) {
	w := workload{"program-loop", 120, 4, false}
	path := generateWorkload(t, w)
	stop := make(chan struct{})
	ready := make(chan struct{})
	done := make(chan struct{})
	var r loopResult
	var err error
	go func() { defer close(done); r, err = runInputLoop(t, path, nil, ready, stop) }()
	select {
	case <-ready:
		close(stop)
	case <-done:
		t.Fatal("program exited before readiness", err)
	case <-time.After(5 * time.Second):
		close(stop)
		t.Fatal("program readiness timeout")
	}
	<-done
	if err != nil || r.Samples == 0 || !r.ColorOutput {
		t.Fatal(r, err)
	}
}
func TestHistoryBlockedCancellation(t *testing.T) {
	for _, r := range measureBlocked(t) {
		t.Logf("%+v", r)
	}
}
