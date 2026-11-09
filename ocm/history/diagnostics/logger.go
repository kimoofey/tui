// Package diagnostics writes allowlisted accounting-operation metadata only.
// Event is deliberately not a generic logger: no raw errors or free-form messages
// can cross the encoding boundary, and native identities are always pseudonymized.
package diagnostics

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

type Kind uint8

const (
	Startup Kind = iota
	Discovery
	SourceLoad
	Collection
	Capture
	Query
	Deletion
	Shutdown
)

var kinds = [...]string{"startup", "discovery", "source_load", "collection", "capture", "query", "deletion", "shutdown"}

type Stage uint8

const (
	Complete Stage = iota
	Begin
	Scope
	Archive
	Native
	Reload
)

var stages = [...]string{"complete", "begin", "scope", "archive", "native", "reload"}

type Code uint8

const (
	OK Code = iota
	Canceled
	Timeout
	Permission
	Missing
	Unsupported
	BindingMismatch
	Stale
	UnknownRole
	Eligibility
	Duplicate
	Failed
)

var codes = [...]string{"ok", "canceled", "timeout", "permission", "missing", "unsupported", "binding_mismatch", "stale", "unknown_role", "unknown_eligibility", "duplicate_identity", "failed"}

type Schema uint8

const (
	UnknownSchema Schema = iota
	V1
	V2
)

func SchemaOf(s string) Schema {
	switch s {
	case "session":
		return V1
	case "session_v2":
		return V2
	}
	return UnknownSchema
}

// Native IDs are input-only correlation keys; they are never encoded verbatim.
type Event struct {
	Kind                 Kind
	Stage                Stage
	Code                 Code
	Schema               Schema
	Duration             time.Duration
	Count                int
	SessionID, MessageID string
}
type record struct {
	Time         string `json:"time"`
	Run          string `json:"run"`
	Version      string `json:"version"`
	Event        string `json:"event"`
	Stage        string `json:"stage"`
	Code         string `json:"code"`
	Schema       string `json:"schema"`
	Milliseconds int64  `json:"duration_ms"`
	Count        int    `json:"count"`
	Session      string `json:"session,omitempty"`
	Message      string `json:"message,omitempty"`
}
type Logger struct {
	mu                 sync.Mutex
	file               *os.File
	lock               *os.File
	path, version, run string
	key                [32]byte
	size, limit        int64
	now                func() time.Time
	failed, closed     bool
}

const DefaultLimit int64 = 1 << 20
const Warning = "History diagnostic logging stopped after a write failure; capture and deletion safeguards remain active."

var versionPattern = regexp.MustCompile(`^(dev|v?[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?)$`)

func Path() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local/state")
	}
	return filepath.Join(base, "ocm/history.log"), nil
}
func Open(path, version string) (l *Logger, err error) {
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err = os.Chmod(dir, 0700); err != nil {
		return nil, err
	}
	if len(version) > 64 || !versionPattern.MatchString(version) {
		version = "unknown"
	}
	l = &Logger{path: path, version: version, limit: DefaultLimit, now: time.Now}
	created := l
	defer func() {
		if err != nil {
			created.Close()
		}
	}()
	if _, err = rand.Read(l.key[:]); err != nil {
		return nil, err
	}
	run := make([]byte, 16)
	if _, err = rand.Read(run); err != nil {
		return nil, err
	}
	l.run = hex.EncodeToString(run)
	l.lock, err = privateFile(path + ".lock")
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(l.lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, fmt.Errorf("history diagnostic log is locked: %w", err)
	}
	for i := 1; i <= 3; i++ {
		p := fmt.Sprintf("%s.%d", path, i)
		info, e := os.Lstat(p)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("diagnostic backup is not a regular file")
		}
		if err = os.Chmod(p, 0600); err != nil {
			return nil, err
		}
	}
	l.file, err = privateFile(path)
	if err != nil {
		return nil, err
	}
	info, err := l.file.Stat()
	if err != nil {
		return nil, err
	}
	l.size = info.Size()
	if l.size > l.limit {
		err = l.rotate()
		if err != nil {
			return nil, err
		}
	}
	return l, nil
}
func privateFile(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_APPEND|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = errors.New("diagnostic file is not regular")
	}
	if err == nil {
		err = f.Chmod(0600)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
func (l *Logger) hash(domain, value string) string {
	if value == "" {
		return ""
	}
	m := hmac.New(sha256.New, l.key[:])
	m.Write([]byte(domain))
	m.Write([]byte{0})
	m.Write([]byte(value))
	return hex.EncodeToString(m.Sum(nil)[:16])
}
func (l *Logger) Record(e Event) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.failed {
		return
	}
	// Invalid enum values are replaced, never formatted from their input.
	kind := "unknown"
	if int(e.Kind) < len(kinds) {
		kind = kinds[e.Kind]
	}
	stage := "unknown"
	if int(e.Stage) < len(stages) {
		stage = stages[e.Stage]
	}
	code := "failed"
	if int(e.Code) < len(codes) {
		code = codes[e.Code]
	}
	schema := "unknown"
	if e.Schema == V1 {
		schema = "session"
	} else if e.Schema == V2 {
		schema = "session_v2"
	}
	r := record{Time: l.now().UTC().Format(time.RFC3339Nano), Run: l.run, Version: l.version, Event: kind, Stage: stage, Code: code, Schema: schema, Milliseconds: max(0, e.Duration.Milliseconds()), Count: max(0, e.Count), Session: l.hash("session", e.SessionID), Message: l.hash("message", e.SessionID+"\x00"+e.MessageID)}
	if e.MessageID == "" {
		r.Message = ""
	}
	b, err := json.Marshal(r)
	if err == nil {
		b = append(b, '\n')
		if l.size+int64(len(b)) > l.limit {
			err = l.rotate()
		}
		if err == nil {
			var n int
			n, err = l.file.Write(b)
			l.size += int64(n)
			if err == nil && n != len(b) {
				err = io.ErrShortWrite
			}
		}
	}
	if err != nil {
		l.failed = true
		if l.file != nil {
			l.file.Close()
		}
	}
}
func (l *Logger) rotate() error {
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return err
		}
		l.file = nil
	}
	if err := os.Remove(l.path + ".3"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for i := 2; i >= 0; i-- {
		from := l.path
		if i > 0 {
			from = fmt.Sprintf("%s.%d", l.path, i)
		}
		to := fmt.Sprintf("%s.%d", l.path, i+1)
		if err := os.Rename(from, to); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	var err error
	l.file, err = privateFile(l.path)
	l.size = 0
	return err
}
func (l *Logger) Warning() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failed {
		return Warning
	}
	return ""
}
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	var errs []error
	if l.file != nil && !l.failed {
		errs = append(errs, l.file.Close())
	}
	if l.lock != nil {
		errs = append(errs, l.lock.Close())
	}
	return errors.Join(errs...)
}
