package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/history/diagnostics"
	"github.com/kimoofey/tui/ocm/session"
)

func TestHistoryDiagnosticsNeverEncodeSensitiveErrorsOrRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.log")
	log, err := diagnostics.Open(path, "dev")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	secret := "PERSONAL_PROMPT user@example.com /Users/person/project token_secret"
	f := &guardedFixture{scopes: map[string][]string{secret: {secret}}}
	f.captureHook = func(context.Context, string) error { return fmt.Errorf("%w: %s", session.ErrHistoryRole, secret) }
	app := New(f, nil, nil)
	app.EnableHistory(f, 0, log)
	result := app.Delete(context.Background(), refs(secret), LoadOptions{})
	if !errors.Is(result.Err, session.ErrHistoryRole) {
		t.Fatal(result)
	}
	cost := 123.45
	f.observations = map[string]session.Observation{secret: {SessionID: secret, Present: true, Binding: session.HistoryBinding{Path: "/Users/private-person/source.sqlite3", Schema: "session_v2"}, Usage: []session.Usage{{Kind: "message", ID: "PRIVATE_MESSAGE_ID", Cost: &cost, Provider: &secret, Model: &secret}}}}
	f.captureHook = nil
	f.deleteHook = func(context.Context) error { return errors.New("native stderr: " + secret) }
	if result = app.Delete(context.Background(), refs(secret), LoadOptions{}); result.Err == nil {
		t.Fatal("native failure lost")
	}
	app.CloseHistory()
	archive, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	browser := NewHistoryBrowser(archive, func(context.Context) (session.Source, error) { return nil, errors.New("source path/config: " + secret) }, nil, LoadOptions{}, log)
	browser.Refresh(context.Background())
	browser.Query(context.Background(), nil)
	browser.Close()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"PERSONAL", "user@example.com", "/Users/person", "token_secret", "native stderr", "source path/config", "private-person", "PRIVATE_MESSAGE_ID", "123.45"} {
		if strings.Contains(string(b), forbidden) {
			t.Fatalf("sensitive log field: %s", forbidden)
		}
	}
	foundRole, foundNative, foundDiscovery, foundQuery := false, false, false, false
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var record map[string]any
		if err = json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record["code"] == "unknown_role" {
			foundRole = true
		}
		if record["stage"] == "native" {
			foundNative = true
		}
		if record["event"] == "discovery" {
			foundDiscovery = true
		}
		if record["event"] == "query" {
			foundQuery = true
		}
	}
	if !foundRole || !foundNative || !foundDiscovery || !foundQuery {
		t.Fatal(string(b))
	}
}
func TestFailedDiagnosticsCannotChangeGuardOrCollection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	log, err := diagnostics.Open(path, "dev")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	// A nonempty third-backup directory makes rotation fail portably, even when
	// tests run as root. No changes to real permissions/files are required.
	if err = os.Mkdir(path+".3", 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(path+".3", "block"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10000 && log.Warning() == ""; i++ {
		log.Record(diagnostics.Event{Kind: diagnostics.Query})
	}
	if log.Warning() == "" {
		t.Fatal("rotation failure not reported")
	}
	f := &guardedFixture{scopes: map[string][]string{"root": {"root"}}, inventory: []string{"root"}}
	app := New(f, nil, nil)
	app.EnableHistory(f, 0, log)
	defer app.CloseHistory()
	app.RequestHistory()
	if status := app.WaitHistory(); status.Err != nil {
		t.Fatal("logging broke collection", status.Err)
	}
	result := app.Delete(context.Background(), refs("root"), LoadOptions{})
	if result.Err != nil || result.Count != 1 {
		t.Fatal("logging broke native action", result)
	}
	app.WaitHistory()
	app.CloseHistory()
	blocked := &guardedFixture{scopes: map[string][]string{"root": {"root"}}, captureFail: "root"}
	guard := New(blocked, nil, nil)
	guard.EnableHistory(blocked, 0, log)
	defer guard.CloseHistory()
	result = guard.Delete(context.Background(), refs("root"), LoadOptions{})
	if result.Err == nil || result.Count != 0 {
		t.Fatal("logging failure bypassed guard", result)
	}
}
func TestDiagnosticClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code diagnostics.Code
	}{{context.Canceled, diagnostics.Canceled}, {context.DeadlineExceeded, diagnostics.Timeout}, {os.ErrPermission, diagnostics.Permission}, {history.ErrBinding, diagnostics.BindingMismatch}, {history.ErrStale, diagnostics.Stale}, {session.ErrHistoryRole, diagnostics.UnknownRole}, {session.ErrHistoryEligibility, diagnostics.Eligibility}, {session.ErrHistoryDuplicate, diagnostics.Duplicate}, {errors.New("private string"), diagnostics.Failed}} {
		if got := diagnosticCode(fmt.Errorf("outer private detail: %w", tc.err)); got != tc.code {
			t.Fatal(got, tc.code)
		}
	}
}
