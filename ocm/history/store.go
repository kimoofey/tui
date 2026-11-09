package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/kimoofey/tui/ocm/session"
	"golang.org/x/sys/unix"
	_ "modernc.org/sqlite"
)

var ErrStale = errors.New("source records removed or known fields lost; saved usage retained")
var ErrBinding = errors.New("history binding mismatch: return to the original source or disable history accepting capture gaps")

type Store struct {
	db   *sql.DB
	lock *os.File
}

func Open(path string) (store *Store, err error) {
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if err = os.Chmod(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	// Resolve symlinks before choosing the sibling lock, including an existing file.
	if resolved, e := filepath.EvalSymlinks(path); e == nil {
		path = resolved
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	} else {
		dir, e := filepath.EvalSymlinks(filepath.Dir(path))
		if e != nil {
			return nil, e
		}
		path = filepath.Join(dir, filepath.Base(path))
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("history archive locked by another instance: %w", err)
	}
	s := &Store{lock: lock}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if e == nil {
		f.Close()
	} else if !errors.Is(e, os.ErrExist) {
		return nil, e
	}
	params := url.Values{"mode": {"rw"}, "_pragma": {"foreign_keys(1)", "synchronous(FULL)", "busy_timeout(5000)"}}
	s.db, err = sql.Open("sqlite", (&url.URL{Scheme: "file", Path: path, RawQuery: params.Encode()}).String())
	if err != nil {
		return nil, err
	}
	s.db.SetMaxOpenConns(1)
	var version int
	if err = s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return nil, fmt.Errorf("read history archive: %w", err)
	}
	if version != 0 && version != 1 && version != 2 {
		return nil, fmt.Errorf("unsupported history schema %d; archive left untouched", version)
	}
	if version == 0 {
		var tables int
		if err = s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'`).Scan(&tables); err != nil {
			return nil, err
		}
		if tables != 0 {
			return nil, errors.New("unrecognized history archive; refusing initialization")
		}
	}
	if err = os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	if _, err = s.db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA synchronous=FULL; PRAGMA busy_timeout=5000;`); err != nil {
		return nil, err
	}
	if version == 0 {
		tx, e := s.db.Begin()
		if e != nil {
			return nil, e
		}
		defer tx.Rollback()
		_, err = tx.Exec(`CREATE TABLE metadata (id INTEGER PRIMARY KEY CHECK(id=1), policy INTEGER NOT NULL, path TEXT NOT NULL, schema TEXT NOT NULL);
 CREATE TABLE sessions (id TEXT PRIMARY KEY, captured INTEGER NOT NULL, status TEXT NOT NULL, v1 INTEGER NOT NULL);
 CREATE TABLE usage (session_id TEXT NOT NULL REFERENCES sessions(id), kind TEXT NOT NULL, id TEXT NOT NULL, timestamp INTEGER, cost REAL, provider TEXT, model TEXT, input INTEGER, output INTEGER, reasoning INTEGER, cache_read INTEGER, cache_write INTEGER, PRIMARY KEY(session_id,kind,id));
 CREATE INDEX usage_timestamp ON usage(timestamp); PRAGMA user_version=1;`)
		if err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
	}
	// Validate existing v1 layouts without modifying or resetting them.
	for _, q := range []string{`SELECT policy,path,schema FROM metadata LIMIT 0`, `SELECT id,captured,status,v1 FROM sessions LIMIT 0`, `SELECT session_id,kind,id,timestamp,cost,provider,model,input,output,reasoning,cache_read,cache_write FROM usage LIMIT 0`} {
		r, e := s.db.Query(q)
		if e != nil {
			return nil, fmt.Errorf("invalid history schema: %w", e)
		}
		r.Close()
	}

	var check string
	if err = s.db.QueryRow(`PRAGMA quick_check`).Scan(&check); err != nil {
		return nil, fmt.Errorf("check history archive: %w", err)
	}
	if check != "ok" {
		return nil, errors.New("corrupt history archive; restore a healthy backup")
	}
	var invalid int
	if err = s.db.QueryRow(`SELECT count(*) FROM metadata WHERE policy!=1 OR schema NOT IN ('session','session_v2')`).Scan(&invalid); err != nil {
		return nil, err
	}
	if invalid != 0 {
		return nil, errors.New("unsupported history accounting policy")
	}
	if version < 2 {
		tx, e := s.db.Begin()
		if e != nil {
			return nil, e
		}
		defer tx.Rollback()
		if _, err = tx.Exec(`CREATE TABLE initial_import (id INTEGER PRIMARY KEY CHECK(id=1), complete INTEGER NOT NULL CHECK(complete IN (0,1))); INSERT INTO initial_import VALUES(1,0); PRAGMA user_version=2;`); err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
	}
	if _, err = s.InitialImportComplete(context.Background()); err != nil {
		return nil, fmt.Errorf("invalid initial import state: %w", err)
	}
	return s, nil
}
func (s *Store) Close() error {
	var e error
	if s.db != nil {
		e = s.db.Close()
	}
	if s.lock != nil {
		e = errors.Join(e, s.lock.Close())
	}
	return e
}
func valid(u session.Usage) bool {
	if u.ID == "" || u.Kind == "" {
		return false
	}
	if u.Cost != nil && (math.IsNaN(*u.Cost) || math.IsInf(*u.Cost, 0) || *u.Cost < 0) {
		return false
	}
	for _, v := range []*int64{u.Input, u.Output, u.Reasoning, u.CacheRead, u.CacheWrite} {
		if v != nil && *v < 0 {
			return false
		}
	}
	return true
}
func scanUsage(rows *sql.Rows) (session.Usage, error) {
	var u session.Usage
	err := rows.Scan(&u.Kind, &u.ID, &u.Timestamp, &u.Cost, &u.Provider, &u.Model, &u.Input, &u.Output, &u.Reasoning, &u.CacheRead, &u.CacheWrite)
	return u, err
}
func loses(a, b session.Usage) bool {
	return a.Timestamp != nil && b.Timestamp == nil || a.Cost != nil && b.Cost == nil || a.Provider != nil && b.Provider == nil || a.Model != nil && b.Model == nil || a.Input != nil && b.Input == nil || a.Output != nil && b.Output == nil || a.Reasoning != nil && b.Reasoning == nil || a.CacheRead != nil && b.CacheRead == nil || a.CacheWrite != nil && b.CacheWrite == nil
}

// Capture atomically reconciles one complete session; stale observations preserve
// every old row, even when they also contain newly observed identities.
func (s *Store) Capture(ctx context.Context, o session.Observation, at time.Time) error {
	if !o.Present {
		return errors.New("session absent; saved usage retained")
	}
	if !filepath.IsAbs(o.Binding.Path) || (o.Binding.Schema != "session" && o.Binding.Schema != "session_v2") || o.SessionID == "" {
		return errors.New("invalid history observation")
	}
	type key struct{ kind, id string }
	incoming := map[key]session.Usage{}
	for _, u := range o.Usage {
		k := key{u.Kind, u.ID}
		if !valid(u) {
			return errors.New("invalid accounting record")
		}
		if _, ok := incoming[k]; ok {
			return errors.New("duplicate usage identity")
		}
		incoming[k] = u
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var path, schema string
	var policy int
	err = tx.QueryRowContext(ctx, `SELECT policy,path,schema FROM metadata WHERE id=1`).Scan(&policy, &path, &schema)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && (policy != 1 || path != o.Binding.Path || schema != o.Binding.Schema) {
		return ErrBinding
	}
	rows, err := tx.QueryContext(ctx, `SELECT kind,id,timestamp,cost,provider,model,input,output,reasoning,cache_read,cache_write FROM usage WHERE session_id=?`, o.SessionID)
	if err != nil {
		return err
	}
	stale := false
	for rows.Next() {
		u, e := scanUsage(rows)
		if e != nil {
			rows.Close()
			return e
		}
		v, ok := incoming[key{u.Kind, u.ID}]
		if !ok || loses(u, v) {
			stale = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if stale {
		if _, err = tx.ExecContext(ctx, `UPDATE sessions SET status=? WHERE id=?`, ErrStale.Error(), o.SessionID); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		return ErrStale
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO metadata VALUES(1,1,?,?)`, o.Binding.Path, o.Binding.Schema); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sessions VALUES(?,?, '',?) ON CONFLICT(id) DO UPDATE SET captured=excluded.captured,status='',v1=excluded.v1`, o.SessionID, at.UnixMilli(), o.ApproximateV1); err != nil {
		return err
	}
	for _, u := range o.Usage {
		_, err = tx.ExecContext(ctx, `INSERT INTO usage VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(session_id,kind,id) DO UPDATE SET timestamp=excluded.timestamp,cost=excluded.cost,provider=excluded.provider,model=excluded.model,input=excluded.input,output=excluded.output,reasoning=excluded.reasoning,cache_read=excluded.cache_read,cache_write=excluded.cache_write`, o.SessionID, u.Kind, u.ID, u.Timestamp, u.Cost, u.Provider, u.Model, u.Input, u.Output, u.Reasoning, u.CacheRead, u.CacheWrite)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RecordFailure persists only a generic status, never source errors containing payloads.
func (s *Store) RecordFailure(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET status='capture failed; saved usage retained' WHERE id=? AND status=''`, id)
	return err
}

// InitialImportComplete is independent of binding and usage, including empty sources.
func (s *Store) InitialImportComplete(ctx context.Context) (bool, error) {
	var complete int
	err := s.db.QueryRowContext(ctx, `SELECT complete FROM initial_import WHERE id=1`).Scan(&complete)
	if err == nil && complete != 0 && complete != 1 {
		return false, errors.New("invalid initial import state")
	}
	return complete == 1, err
}
func (s *Store) CompleteInitialImport(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE initial_import SET complete=1 WHERE id=1`)
	return err
}
