//go:build !mock

package ocm

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kimoofey/tui/ocm/db"
)

func testExecutable(t *testing.T, script string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "bin with 'quotes' $and `ticks`")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "opencode")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func readNULFields(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
}

func TestTerminalCommand_PreservesStorageAndQuoting(t *testing.T) {
	record := filepath.Join(t.TempDir(), "record")
	executable := testExecutable(t, `
printf '%s\000' "$@" > "$OCM_TEST_RECORD"
printf '%s\000' "$OPENCODE_DB" "$XDG_DATA_HOME" "$XDG_CONFIG_HOME" "${OPENCODE_CONFIG_DIR-unset}" > "$OCM_TEST_RECORD.env"
`)
	t.Setenv("XDG_CONFIG_HOME", "/tmp/config 'quotes' $dollar `ticks` \\\nnew line")
	// OPENCODE_CONFIG_DIR is unset in OCM, even if a terminal has set it.
	original, hadOriginal := os.LookupEnv("OPENCODE_CONFIG_DIR")
	if err := os.Unsetenv("OPENCODE_CONFIG_DIR"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadOriginal {
			_ = os.Setenv("OPENCODE_CONFIG_DIR", original)
		}
	})
	storage := db.StorageContext{
		DBPath:  "/tmp/db 'quote' $(literal) `literal` \\.db",
		DataDir: "/tmp/data 'quote' $literal/opencode", Executable: executable,
		V2CLI: true, Standalone: true,
	}
	id := "ses_'$(literal)`literal`\n\\\""
	for _, terminal := range []string{"terminal", "iterm2", "ghostty", "custom-terminal --"} {
		t.Run(terminal, func(t *testing.T) {
			launch := terminalCommand(terminal, storage, id)
			var command *exec.Cmd
			switch terminal {
			case "terminal", "iterm2":
				marker := " to do script "
				if terminal == "iterm2" {
					marker = " profile command "
				}
				_, literal, ok := strings.Cut(launch.Args[2], marker)
				if !ok {
					t.Fatalf("unexpected AppleScript: %q", launch.Args)
				}
				shell, err := strconv.Unquote(literal)
				if err != nil {
					t.Fatal(err)
				}
				command = exec.Command("/bin/sh", "-c", shell)
			case "ghostty":
				command = exec.Command("/bin/sh", "-c", launch.Args[len(launch.Args)-1])
			default:
				command = exec.Command(launch.Args[2], launch.Args[3:]...)
			}
			// Simulate a terminal whose path environment differs from OCM's.
			command.Env = []string{
				"PATH=/usr/bin:/bin", "OCM_TEST_RECORD=" + record,
				"OPENCODE_DB=/wrong.db", "XDG_DATA_HOME=/wrong", "XDG_CONFIG_HOME=/wrong",
				"OPENCODE_CONFIG_DIR=/terminal-only",
			}
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("launch: %v: %s", err, output)
			}
			if got := readNULFields(t, record); !reflect.DeepEqual(got, []string{"-s", id, "--standalone"}) {
				t.Fatalf("resume args=%q", got)
			}
			wantEnv := []string{storage.DBPath, filepath.Dir(storage.DataDir), os.Getenv("XDG_CONFIG_HOME"), "unset"}
			if got := readNULFields(t, record+".env"); !reflect.DeepEqual(got, wantEnv) {
				t.Fatalf("environment=%q, want=%q", got, wantEnv)
			}
		})
	}
}

func TestRunDelete_StorageAndCLICapabilities(t *testing.T) {
	record := filepath.Join(t.TempDir(), "record")
	executable := testExecutable(t, `
printf '%s\000' "$@" > "$OCM_TEST_RECORD"
printf '%s\000' "$OPENCODE_DB" "$XDG_DATA_HOME" > "$OCM_TEST_RECORD.env"
`)
	t.Setenv("OCM_TEST_RECORD", record)
	t.Setenv("OPENCODE_DB", "wrong.db")
	t.Setenv("XDG_DATA_HOME", "/wrong")
	for _, tt := range []struct {
		name           string
		v2, standalone bool
	}{
		{"V1", false, true}, {"V2 default", true, false}, {"V2 custom", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			storage := db.StorageContext{DBPath: "/tmp/db with spaces.db", DataDir: "/tmp/data/opencode", Executable: executable, V2CLI: tt.v2, Standalone: tt.standalone}
			if err := runDelete(context.Background(), storage, "ses_'literal'"); err != nil {
				t.Fatal(err)
			}
			want := []string{"session", "delete", "ses_'literal'"}
			if tt.v2 && tt.standalone {
				want = append(want, "--standalone")
			}
			if got := readNULFields(t, record); !reflect.DeepEqual(got, want) {
				t.Fatalf("delete args=%q, want=%q", got, want)
			}
			if got := readNULFields(t, record+".env"); !reflect.DeepEqual(got, []string{storage.DBPath, "/tmp/data"}) {
				t.Fatalf("delete environment=%q", got)
			}
		})
	}
}

func TestRunDelete_FailureAndTimeout(t *testing.T) {
	executable := testExecutable(t, "echo 'session unavailable' >&2\nexit 7\n")
	storage := db.StorageContext{Executable: executable, DBPath: "/tmp/test.db", DataDir: "/tmp/opencode"}
	if err := runDelete(context.Background(), storage, "missing"); err == nil || !strings.Contains(err.Error(), "session unavailable") {
		t.Fatalf("error=%v", err)
	}
	storage.Executable = testExecutable(t, "exec /bin/sleep 10\n")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := runDelete(ctx, storage, "missing"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error=%v", err)
	}
}

// Invoked only in a subprocess by the fake OpenCode executable below. It
// simulates cascading deletion against the disposable test database.
func TestDeleteProcessHelper(t *testing.T) {
	if os.Getenv("OCM_DELETE_HELPER") != "1" {
		return
	}
	_, args, ok := strings.Cut(strings.Join(os.Args, "\x00"), "--\x00")
	parts := strings.Split(args, "\x00")
	if !ok || len(parts) < 3 || parts[0] != "session" || parts[1] != "delete" {
		t.Fatalf("unexpected helper args: %q", parts)
	}
	if parts[2] == "fail" {
		t.Fatal("simulated delete failure")
	}
	uri := (&url.URL{Scheme: "file", Path: os.Getenv("OPENCODE_DB"), RawQuery: "mode=rw"}).String()
	writer, err := sql.Open("sqlite", uri)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	if _, err := writer.Exec(`PRAGMA foreign_keys = ON;
		DELETE FROM session_v2 WHERE id = ? OR parent_id = ?`, parts[2], parts[2]); err != nil {
		t.Fatal(err)
	}
}

func deleteTestStore(t *testing.T) *db.Store {
	t.Helper()
	dataDir := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dataDir, "opencode.db")
	writer, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	_, err = writer.Exec(`
		CREATE TABLE session_v2 (id TEXT PRIMARY KEY, parent_id TEXT, fork_session_id TEXT, title TEXT,
			directory TEXT, time_created INTEGER, time_updated INTEGER, cost REAL);
		CREATE TABLE session_message (session_id TEXT REFERENCES session_v2(id) ON DELETE CASCADE,
			type TEXT, time_created INTEGER, data TEXT);
		CREATE TABLE event (aggregate_id TEXT, type TEXT, created INTEGER, data TEXT);
		INSERT INTO session_v2 VALUES
			('root', NULL, NULL, 'Root', '/p', 1, 3, 1),
			('child', 'root', NULL, 'Child', '/p', 1, 2, 1),
			('keep', NULL, NULL, 'Keep', '/q', 1, 1, 1);
		INSERT INTO session_message VALUES ('root', 'assistant', ?, '{"cost":1}'),
			('child', 'assistant', ?, '{"cost":2}'), ('keep', 'assistant', ?, '{"cost":3}');`,
		time.Now().UnixMilli()-1000, time.Now().UnixMilli()-1000, time.Now().UnixMilli()-1000)
	if err != nil {
		t.Fatal(err)
	}
	executable := testExecutable(t, `exec "$OCM_TEST_BINARY" -test.run='^TestDeleteProcessHelper$' -- "$@"`)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCM_TEST_BINARY", binary)
	t.Setenv("OCM_DELETE_HELPER", "1")
	store, err := db.OpenStorage(db.StorageContext{DBPath: path, DataDir: dataDir, Executable: executable, V2CLI: true, Standalone: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestDeleteSession_RefreshesChildrenCountsAndCosts(t *testing.T) {
	store := deleteTestStore(t)
	msg := deleteSession("root", store, false, CostPeriod("year"))().(deletedMsg)
	if msg.err != nil {
		t.Fatal(msg.err)
	}
	if msg.data.newTotal != 1 || len(msg.data.sessions) != 1 || msg.data.sessions[0].ID != "keep" || msg.data.periodCost != 3 {
		t.Fatalf("reload after delete: %+v", msg.data)
	}
	model := New(Options{Sessions: []db.Session{{ID: "root"}, {ID: "child"}, {ID: "keep"}}, TotalCount: 3}, 100, 30)
	updated, _ := model.Update(msg)
	got := updated.(Model)
	if len(got.sessions) != 1 || got.totalSessions != 1 || got.periodCost != 3 || !got.statusOK {
		t.Fatalf("model after delete: total=%d, sessions=%v, status=%q", got.totalSessions, got.sessions, got.status)
	}
	last := deleteSession("keep", store, false, "")().(deletedMsg)
	if last.err != nil || last.data.newTotal != 0 || len(last.data.sessions) != 0 {
		t.Fatalf("last deletion: %+v", last)
	}
	updated, _ = got.Update(last)
	if model := updated.(Model); model.totalSessions != 0 || len(model.sessions) != 0 {
		t.Fatalf("last deletion left rows/count: %v/%d", model.sessions, model.totalSessions)
	}
}

func TestBulkDelete_RefreshesOnPartialFailure(t *testing.T) {
	store := deleteTestStore(t)
	msg := bulkDeleteSessions([]string{"root", "fail"}, store, true, "")().(bulkDeletedMsg)
	if msg.err == nil || msg.count != 1 || msg.data.err != nil || msg.data.newTotal != 1 || len(msg.data.sessions) != 1 {
		t.Fatalf("bulk result: %+v", msg)
	}
	model := New(Options{Sessions: []db.Session{{ID: "root"}, {ID: "child"}, {ID: "keep"}}, TotalCount: 3}, 100, 30)
	model.selected["root"] = true
	updated, _ := model.Update(msg)
	got := updated.(Model)
	if len(got.sessions) != 1 || got.totalSessions != 1 || got.statusOK || len(got.selected) != 0 {
		t.Fatalf("partial failure left stale data: total=%d, sessions=%v, status=%q", got.totalSessions, got.sessions, got.status)
	}
}

func TestModel_CustomStorageCannotPrune(t *testing.T) {
	store := deleteTestStore(t)
	storage := store.Storage()
	storage.DataDir = filepath.Join(t.TempDir(), "opencode")
	custom, err := db.OpenStorage(storage)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = custom.Close() }()
	model := New(Options{Store: custom, OrphanCount: 42}, 100, 30)
	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'p'})
	got := updated.(Model)
	if cmd != nil || got.prunePhase || !strings.Contains(got.status, "pruning unavailable") {
		t.Fatalf("custom prune: phase=%v, status=%q", got.prunePhase, got.status)
	}
}
