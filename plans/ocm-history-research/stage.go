package research

// Research only: separate per-session source snapshots and disposable disk stages.
// No source paging: every sealed stage comes from one completed read transaction.
import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
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

	// Bound batches by rows and serialized payload (or one larger, field-capped
	// record, flushed immediately). Every batch stays inside
	// this one destination transaction; only the final seal makes it reusable.
	const batchRows = 128
	const batchPayload = 64 << 10
	args := make([]any, 0, batchRows*2)
	pendingBytes := 0
	flush := func() error {
		if len(args) == 0 {
			return nil
		}
		values := strings.TrimSuffix(strings.Repeat("(?,?),", len(args)/2), ",")
		_, e := tx.ExecContext(ctx, `INSERT INTO staged_usage VALUES `+values, args...)
		clear(args)
		args = args[:0]
		pendingBytes = 0
		return e
	}

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
		key := usageKey(u)
		size := len(raw) + len(key)
		stagedBytes += size
		if stagedBytes > 64<<20 {
			return errors.New("stage accounting payload exceeds 64 MiB")
		}
		if pendingBytes+size > batchPayload {
			if e = flush(); e != nil {
				return e
			}
		}
		args = append(args, key, string(raw))
		pendingBytes += size
		if len(args)/2 >= batchRows || pendingBytes >= batchPayload {
			return flush()
		}
		return nil
	}
	o, err := extract(ctx, source, opts)
	if err != nil {
		return nil, err
	}
	if err = flush(); err != nil {
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
