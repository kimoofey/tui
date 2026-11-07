package research

// Research only: separate per-session source snapshots and disposable disk stages.
// No source paging: every sealed stage comes from one completed read transaction.
import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const stageSchema = `CREATE TABLE staged_usage (key TEXT PRIMARY KEY, accounting TEXT NOT NULL) WITHOUT ROWID;
CREATE TABLE seal (id INTEGER PRIMARY KEY CHECK(id=1), metadata TEXT NOT NULL);`

// stageSession writes to a fresh private destination owned by the caller. The
// source deadline includes SQLite parsing and destination inserts. A timeout,
// failed source commit, or failed seal rolls back the stage. No archive is touched.
// The stage is not correction authority, lineage evidence, or a deletion permit.
func stageSession(ctx context.Context, source, stage *sql.DB, opts extractOptions) (*Observation, error) {
	if opts.Session == "" {
		return nil, errors.New("staging requires one explicit session")
	}
	tx, err := stage.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO staged_usage VALUES(?,?)`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stmt.Close() }()
	stagedBytes := 0
	opts.Sink = func(u Usage) error {
		// Accounting metadata can itself be hostile; never stage unbounded labels.
		for _, value := range []string{u.Session, u.Kind, u.ID, u.Provider, u.Model, u.Variant} {
			if len(value) > 4096 {
				return errors.New("stage metadata field exceeds 4 KiB")
			}
		}
		raw, e := json.Marshal(u)
		if e != nil {
			return e
		}
		stagedBytes += len(raw) + len(usageKey(u))
		if stagedBytes > 64<<20 {
			return errors.New("stage accounting payload exceeds 64 MiB")
		}
		_, e = stmt.ExecContext(ctx, usageKey(u), string(raw))
		return e
	}
	o, err := extract(ctx, source, opts)
	if err != nil {
		return nil, err
	}
	// A missing session is loss/ambiguity, not a complete empty replacement.
	if !o.Sessions[opts.Session] {
		return nil, errors.New("session absent from snapshot")
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO seal VALUES(1,?)`, string(raw)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return o, nil
}

// collectSession tries a small ordinary observation, then starts a NEW snapshot
// for disk staging. It never concatenates the two attempts. Caller schedules
// retries/yields; exhausted exceptional budget means deferred, never complete.
func collectSession(ctx context.Context, source, stage *sql.DB, session string, ordinary, exceptional time.Duration, rows int) (*Observation, bool, error) {
	first, cancel := context.WithTimeout(ctx, ordinary)
	o, err := extract(first, source, extractOptions{Session: session, MaxRows: rows})
	cancel()
	if err == nil {
		if !o.Sessions[session] {
			return nil, false, errors.New("session absent from snapshot")
		}
		return o, false, nil
	}
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	if !errors.Is(err, errBounded) && !errors.Is(err, context.DeadlineExceeded) {
		return nil, false, err
	}
	retry, cancel := context.WithTimeout(ctx, exceptional)
	defer cancel()
	o, err = stageSession(retry, source, stage, extractOptions{Session: session})
	return o, true, err
}
