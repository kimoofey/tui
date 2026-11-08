//go:build !mock

package opencode

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	terminalpkg "github.com/kimoofey/tui/internal/terminal"
	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
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
	t.Setenv("ANTHROPIC_API_KEY", "provider-secret")
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
	storage := storageContext{
		DBPath:  "/tmp/db 'quote' $(literal) `literal` \\.db",
		DataDir: "/tmp/data 'quote' $literal/opencode", Executable: executable,
		V2CLI: true,
	}
	id := "ses_'$(literal)`literal`\n\\\""
	for _, terminal := range []string{"terminal", "iterm2", "ghostty", "custom-terminal --"} {
		t.Run(terminal, func(t *testing.T) {
			spec, err := (&Store{storage: storage}).Resume(context.Background(), id)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(strings.Join(spec.Args, " "), "provider-secret") {
				t.Fatal("provider credential serialized")
			}
			launch, err := terminalpkg.Build(terminal, spec)
			if err != nil {
				t.Fatal(err)
			}
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
		name string
		v2   bool
	}{
		{"V1", false}, {"V2 default", true}, {"V2 custom", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			storage := storageContext{DBPath: "/tmp/db with spaces.db", DataDir: "/tmp/data/opencode", Executable: executable, V2CLI: tt.v2}
			if tt.name == "V2 default" {
				home, err := os.UserHomeDir()
				if err != nil {
					t.Fatal(err)
				}
				storage.DataDir = filepath.Join(home, ".local", "share", "opencode")
				storage.DBPath = filepath.Join(storage.DataDir, "opencode.db")
			}
			if err := runDelete(context.Background(), storage, "ses_'literal'"); err != nil {
				t.Fatal(err)
			}
			want := []string{"session", "delete", "ses_'literal'"}
			if tt.v2 {
				want = append(want, "--standalone")
			}
			if got := readNULFields(t, record); !reflect.DeepEqual(got, want) {
				t.Fatalf("delete args=%q, want=%q", got, want)
			}
			if got := readNULFields(t, record+".env"); !reflect.DeepEqual(got, []string{storage.DBPath, filepath.Dir(storage.DataDir)}) {
				t.Fatalf("delete environment=%q", got)
			}
			wantResume := []string{"-s", "ses_resume"}
			if tt.v2 {
				wantResume = append(wantResume, "--standalone")
			}
			spec, err := (&Store{storage: storage}).Resume(context.Background(), "ses_resume")
			if err != nil {
				t.Fatal(err)
			}
			resume := spec.Args
			if got := resume[len(resume)-len(wantResume):]; !reflect.DeepEqual(got, wantResume) {
				t.Fatalf("resume args=%q, want=%q", got, wantResume)
			}
		})
	}
}

func TestRunDelete_FailureAndTimeout(t *testing.T) {
	executable := testExecutable(t, "echo 'session unavailable' >&2\nexit 7\n")
	storage := storageContext{Executable: executable, DBPath: "/tmp/test.db", DataDir: "/tmp/opencode"}
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

func TestGuardedNativeActionUsesOriginalRouting(t *testing.T) {
	s, w := createV2Store(t)
	execTestSQL(t, w, `PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000`)
	execTestSQL(t, w, `INSERT INTO session_v2 VALUES('root','Root','/p',1000,2000,0,NULL,NULL),('child','Child','/p',1000,2000,0,'root',NULL);
 INSERT INTO session_message VALUES('root-usage','root','assistant',1,1000,1000,'{"cost":2}'),('child-usage','child','assistant',1,1000,1000,'{"cost":1}')`)
	record := filepath.Join(t.TempDir(), "native-args")
	t.Setenv("OCM_GUARD_RECORD", record)
	s.storage.Executable = testExecutable(t, `printf '%s\000' "$@" "$OPENCODE_DB" > "$OCM_GUARD_RECORD"`)
	s.storage.V2CLI = true
	archive, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	app := service.New(s, nil, nil)
	app.EnableHistory(archive, 0)
	defer app.CloseHistory()
	result := app.Delete(context.Background(), []session.Ref{{Source: s.Info().ID, ID: "root"}, {Source: s.Info().ID, ID: "child"}}, service.LoadOptions{})
	if result.Err != nil || result.Count != 1 {
		t.Fatal(result)
	}
	want := []string{"session", "delete", "root", "--standalone", s.Path()}
	if got := readNULFields(t, record); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	r, err := archive.Query(context.Background(), nil)
	if err != nil || r.Cost == nil || *r.Cost != 3 {
		t.Fatal(r, err)
	}
	if err = os.Remove(record); err != nil {
		t.Fatal(err)
	}
	execTestSQL(t, w, `UPDATE session_message SET type='unrecognized' WHERE session_id='child'`)
	result = app.Delete(context.Background(), []session.Ref{{Source: s.Info().ID, ID: "root"}}, service.LoadOptions{})
	if result.Err == nil {
		t.Fatal("unknown type allowed native delete")
	}
	if _, err = os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("native action launched after failed capture", err)
	}
}
