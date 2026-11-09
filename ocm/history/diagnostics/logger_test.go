package diagnostics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func readRecords(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var r map[string]any
		if err = json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}
func TestRotationPrivacyPermissionsAndCorrelation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ocm/history.log")
	l, err := Open(path, "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	l.limit = 1024
	l.now = func() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 0, time.FixedZone("test", 3600)) }
	secret := "PRIVATE_Conversation /Users/private-user/ token=secret@example.com"
	event := Event{Kind: Capture, Code: UnknownRole, Schema: V2, SessionID: secret, MessageID: secret, Duration: time.Second, Count: 7}
	l.Record(event)
	l.Record(event)
	first := readRecords(t, path)
	if len(first) != 2 || first[0]["session"] != first[1]["session"] || first[0]["message"] != first[1]["message"] || first[0]["session"] == first[0]["message"] {
		t.Fatal(first)
	}
	if first[0]["code"] != "unknown_role" || first[0]["time"] != "2026-11-08T11:00:00Z" {
		t.Fatal(first)
	}
	for i := 0; i < 30; i++ {
		l.Record(event)
	}
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{}
	for _, key := range []string{"time", "run", "version", "event", "stage", "code", "schema", "duration_ms", "count", "session", "message"} {
		allowed[key] = true
	}
	for i := 0; i <= 3; i++ {
		p := path
		if i > 0 {
			p = fmt.Sprintf("%s.%d", path, i)
		}
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 || info.Size() > l.limit {
			t.Fatal("permissions/size", p, info.Mode(), info.Size())
		}
		b, _ := os.ReadFile(p)
		if strings.Contains(string(b), "PRIVATE") || strings.Contains(string(b), "private-user") || strings.Contains(string(b), "secret@example.com") {
			t.Fatal("sensitive identity leaked")
		}
		for _, r := range readRecords(t, p) {
			for key := range r {
				if !allowed[key] {
					t.Fatal("unapproved field", key)
				}
			}
		}
	}
	for _, p := range []string{filepath.Dir(path), path + ".lock"} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0600)
		if info.IsDir() {
			want = 0700
		}
		if info.Mode().Perm() != want {
			t.Fatal(info.Mode())
		}
	}
	if _, err = os.Stat(path + ".4"); !os.IsNotExist(err) {
		t.Fatal("unbounded backups")
	}
	next, err := Open(path, "dev")
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	next.Record(event)
	records := readRecords(t, path)
	last := records[len(records)-1]
	if last["run"] == first[0]["run"] || last["session"] == first[0]["session"] {
		t.Fatal("identifiers correlate across runs")
	}
}
func TestFailureDisablesLoggingAndCloseReleasesLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	l, err := Open(path, "PRIVATE_VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Open(path, "dev"); err == nil {
		other.Close()
		t.Fatal("concurrent logger accepted")
	}
	l.Record(Event{Kind: Kind(255), Stage: Stage(255), Code: Code(255), Schema: Schema(255)})
	r := readRecords(t, path)[0]
	if r["version"] != "unknown" || r["event"] != "unknown" || r["schema"] != "unknown" {
		t.Fatal(r)
	}
	l.file.Close()
	l.Record(Event{Kind: Capture})
	if l.Warning() != Warning {
		t.Fatal("missing failure status")
	}
	before, _ := os.ReadFile(path)
	l.Record(Event{Kind: Query})
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("logging continued after failure")
	}
	l.Close()
	next, err := Open(path, "dev")
	if err != nil {
		t.Fatal("failed logger leaked lock", err)
	}
	next.Close()
}
func TestUnsafeFilesRejectedAndInitReleasesLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("private target"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if l, err := Open(path, "dev"); err == nil {
		l.Close()
		t.Fatal("followed log symlink")
	}
	os.Remove(path)
	// Failed initialization must release the already acquired sibling lock.
	l, err := Open(path, "dev")
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	b, _ := os.ReadFile(target)
	if string(b) != "private target" {
		t.Fatal("modified symlink target")
	}
}
func TestConcurrentEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	l, err := Open(path, "dev")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				l.Record(Event{Kind: Query})
				_ = l.Warning()
			}
		}()
	}
	wg.Wait()
	if len(readRecords(t, path)) != 100 {
		t.Fatal("lost/interleaved records")
	}
}
func TestPathFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")
	p, err := Path()
	if err != nil || p != filepath.Join(os.Getenv("HOME"), ".local/state/ocm/history.log") {
		t.Fatal(p, err)
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	p, err = Path()
	if err != nil || p != filepath.Join(os.Getenv("XDG_STATE_HOME"), "ocm/history.log") {
		t.Fatal(p, err)
	}
}
