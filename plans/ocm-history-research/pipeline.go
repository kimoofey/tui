package research

// Isolated capacity prototype. Float accounting and this schema are NOT the
// proposed production decimal/schema contract. No native identity is inferred.
import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"
)

const pipelineSchema = `
CREATE TABLE accepted(session TEXT PRIMARY KEY,generation INTEGER NOT NULL,metadata TEXT NOT NULL,status TEXT NOT NULL);
CREATE TABLE contributions(session TEXT NOT NULL,key TEXT NOT NULL,accounting TEXT NOT NULL,PRIMARY KEY(session,key)) WITHOUT ROWID;
CREATE TABLE session_daily(session TEXT NOT NULL,day TEXT,provider TEXT,model TEXT,variant TEXT,cost REAL,known_cost INTEGER,unknown_cost INTEGER,steps INTEGER,tokens TEXT,unknown_tokens TEXT,quality TEXT,PRIMARY KEY(session,day,provider,model,variant)) WITHOUT ROWID;
CREATE TABLE pending(key TEXT PRIMARY KEY,accounting TEXT NOT NULL) WITHOUT ROWID;`

var errStalePublication = errors.New("stale publication generation")

// inventoryPage uses a dedicated short snapshot for discovery, not an assertion
// of source-wide completeness. Native writers may add IDs behind the cursor.
func inventoryPage(ctx context.Context, source *sql.DB, after string, limit int) ([]string, error) {
	if limit < 1 || limit > 1000 {
		return nil, errors.New("invalid inventory page limit")
	}
	tx, err := source.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var table string
	if err = tx.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name IN ('session_v2','session') ORDER BY CASE name WHEN 'session_v2' THEN 0 ELSE 1 END LIMIT 1`).Scan(&table); err != nil {
		return nil, err
	}
	// Validate even an empty authoritative layout; no capture will run when
	// inventory is empty, so missing usage tables must not become complete zero.
	probe := `SELECT s.id,s.time_created,m.id,m.session_id,m.time_created,m.data FROM session s LEFT JOIN message m ON 0 LIMIT 0`
	if table == "session_v2" {
		probe = `SELECT s.id,s.time_created,s.fork_session_id,m.id,m.session_id,m.type,m.time_created,m.data,e.id,e.aggregate_id,e.type,e.created,e.data FROM session_v2 s LEFT JOIN session_message m ON 0 LEFT JOIN event e ON 0 LIMIT 0`
	}
	schemaRows, e := tx.QueryContext(ctx, probe)
	if e != nil {
		return nil, fmt.Errorf("unsupported inventory schema: %w", e)
	}
	if e = schemaRows.Close(); e != nil {
		return nil, e
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM `+table+` WHERE id>? ORDER BY id LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	closeErr := rows.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return ids, nil
}

type publishOptions struct {
	Expected     int64
	Corrections  map[string]bool // explicit synthetic authority; never discovered from IDs
	BeforeWrite  func()          // instrumentation only, before accepted archive rows change
	BeforeCommit func() error    // fault/cancellation injection, outside performance runs
}

// publishSession consumes either a complete in-memory observation or a sealed
// stage. Staged items stream to a transactional scratch table; no huge Items map is built.
// Contribution replacement, generation and per-session aggregates commit together.
func publishSession(ctx context.Context, archive *sql.DB, sid string, o *Observation, stage *sql.DB, opts publishOptions) (status string, err error) {
	if stage != nil {
		var raw string
		if err = stage.QueryRowContext(ctx, `SELECT metadata FROM seal WHERE id=1`).Scan(&raw); err != nil {
			return "", err
		}
		o = &Observation{}
		if err = json.Unmarshal([]byte(raw), o); err != nil {
			return "", err
		}
	} else if o == nil || !o.Complete {
		return "", errors.New("incomplete observation")
	}
	if o.Scope != "session:"+sid || !o.Sessions[sid] {
		return "", errors.New("publication scope mismatch")
	}
	tx, err := archive.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	var generation int64
	var previous string
	err = tx.QueryRowContext(ctx, `SELECT generation,metadata FROM accepted WHERE session=?`, sid).Scan(&generation, &previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if generation != opts.Expected {
		return "", errStalePublication
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM pending`); err != nil {
		return "", err
	}
	grouped := map[bucketKey]bucket{}
	args := make([]any, 0, 256)
	flush := func() error {
		if len(args) == 0 {
			return nil
		}
		q := `INSERT INTO pending VALUES `
		for i := 0; i < len(args)/2; i++ {
			if i > 0 {
				q += ","
			}
			q += "(?,?)"
		}
		_, e := tx.ExecContext(ctx, q, args...)
		clear(args)
		args = args[:0]
		return e
	}
	accept := func(key, raw string) error {
		var u Usage
		if e := json.Unmarshal([]byte(raw), &u); e != nil {
			return e
		}
		if u.Session != sid || usageKey(u) != key {
			return errors.New("stage identity mismatch")
		}
		one := &Observation{Quality: o.Quality, Items: map[string]Usage{key: u}}
		for k, b := range aggregate(one, time.UTC, nil, nil) {
			c := grouped[k]
			c.Cost += b.Cost
			c.Count += b.Count
			c.KnownCost += b.KnownCost
			c.UnknownCost += b.UnknownCost
			c.Quality = b.Quality
			for j := range c.Tokens {
				c.Tokens[j] += b.Tokens[j]
				c.UnknownTokens[j] += b.UnknownTokens[j]
			}
			grouped[k] = c
		}
		args = append(args, key, raw)
		if len(args) == 256 {
			return flush()
		}
		return nil
	}
	if stage != nil {
		rows, e := stage.QueryContext(ctx, `SELECT key,accounting FROM staged_usage`)
		if e != nil {
			return "", e
		}
		for rows.Next() {
			var key, raw string
			if err = rows.Scan(&key, &raw); err == nil {
				err = accept(key, raw)
			}
			if err != nil {
				break
			}
		}
		if err == nil {
			err = rows.Err()
		}
		closeErr := rows.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return "", err
		}
	} else {
		for key, u := range o.Items {
			raw, e := json.Marshal(u)
			if e != nil {
				return "", e
			}
			if err = accept(key, string(raw)); err != nil {
				return "", err
			}
		}
	}
	if err = flush(); err != nil {
		return "", err
	}
	status = "accepted"
	if generation > 0 {
		var old Observation
		if err = json.Unmarshal([]byte(previous), &old); err != nil {
			return "", err
		}
		if old.Schema != o.Schema || old.Policy != o.Policy {
			status = "unresolved-migration"
		}
		var lost int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM contributions c LEFT JOIN pending p ON p.key=c.key WHERE c.session=? AND p.key IS NULL`, sid).Scan(&lost); err != nil {
			return "", err
		}
		if lost > 0 {
			status = "unresolved-record-loss"
		}
		rows, e := tx.QueryContext(ctx, `SELECT c.key FROM contributions c JOIN pending p ON p.key=c.key WHERE c.session=? AND c.accounting!=p.accounting`, sid)
		if e != nil {
			return "", e
		}
		changed := false
		for rows.Next() {
			var key string
			if err = rows.Scan(&key); err != nil {
				break
			}
			changed = true
			if !opts.Corrections[key] && status == "accepted" {
				status = "unresolved-correction"
			}
		}
		if err == nil {
			err = rows.Err()
		}
		_ = rows.Close()
		if err != nil {
			return "", err
		}
		var added int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM pending p LEFT JOIN contributions c ON c.session=? AND c.key=p.key WHERE c.key IS NULL`, sid).Scan(&added); err != nil {
			return "", err
		}
		if len(o.Diagnostics) > 0 && (changed || added > 0) {
			status = "unresolved-fields"
		}
		if status == "accepted" && !changed && added == 0 {
			status = "unchanged"
		}
	}
	if opts.BeforeWrite != nil {
		opts.BeforeWrite()
	}
	if status != "accepted" {
		if _, err = tx.ExecContext(ctx, `UPDATE accepted SET status=? WHERE session=?`, status, sid); err != nil {
			return "", err
		}
	} else {
		for _, table := range []string{"contributions", "session_daily"} {
			if _, err = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE session=?`, sid); err != nil {
				return "", err
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO contributions SELECT ?,key,accounting FROM pending`, sid); err != nil {
			return "", err
		}
		for k, b := range grouped {
			tokens, _ := json.Marshal(b.Tokens)
			unknown, _ := json.Marshal(b.UnknownTokens)
			if _, err = tx.ExecContext(ctx, `INSERT INTO session_daily VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, sid, k.Day, k.Provider, k.Model, k.Variant, b.Cost, b.KnownCost, b.UnknownCost, b.Count, string(tokens), string(unknown), b.Quality); err != nil {
				return "", err
			}
		}
		meta := *o
		meta.Items = nil
		raw, e := json.Marshal(meta)
		if e != nil {
			return "", e
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO accepted VALUES(?,?,?,?) ON CONFLICT(session) DO UPDATE SET generation=excluded.generation,metadata=excluded.metadata,status=excluded.status`, sid, generation+1, string(raw), status); err != nil {
			return "", err
		}
	}
	// Scratch lives in the archive so file/WAL accounting includes its pages.
	if _, err = tx.ExecContext(ctx, `DELETE FROM pending`); err != nil {
		return "", err
	}
	if opts.BeforeCommit != nil {
		if err = opts.BeforeCommit(); err != nil {
			return "", err
		}
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	return status, tx.Commit()
}

type pipelineTiming struct {
	InventoryNS, CaptureNS, PublishNS, TotalNS, MaxAttemptNS, MaxSessionNS, MaxBatchNS int64
	Sessions, Staged, Deferred, Yields                                                 int
}

type pipelineHooks struct {
	Phase         func(string)
	BeforePublish func()
	StageFile     func(string)
	Yield         func()
}

// sweep intentionally rescans old sessions: timestamps cannot prove freshness.
// Every page is bounded and each accepted session has its own atomic publication.
func sweep(ctx context.Context, source, archive *sql.DB, newStage func() (*sql.DB, string, error), hooks pipelineHooks) (r pipelineTiming, err error) {
	start := time.Now()
	defer func() { r.TotalNS = time.Since(start).Nanoseconds() }()
	phase := func(s string) {
		if hooks.Phase != nil {
			hooks.Phase(s)
		}
	}
	cursor := ""
	batch := time.Now()
	inBatch := 0
	yield := func() {
		r.MaxBatchNS = max(r.MaxBatchNS, time.Since(batch).Nanoseconds())
		r.Yields++
		if hooks.Yield != nil {
			hooks.Yield()
		} else {
			runtime.Gosched()
		}
		batch = time.Now()
		inBatch = 0
	}
	for {
		phase("inventory")
		at := time.Now()
		pageCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
		ids, e := inventoryPage(pageCtx, source, cursor, 100)
		cancel()
		r.InventoryNS += time.Since(at).Nanoseconds()
		r.MaxAttemptNS = max(r.MaxAttemptNS, time.Since(at).Nanoseconds())
		if e != nil {
			return r, e
		}
		if len(ids) == 0 {
			break
		}
		for _, sid := range ids {
			if e = ctx.Err(); e != nil {
				return r, e
			}
			if inBatch >= 10 || time.Since(batch) >= 25*time.Millisecond {
				yield()
			}
			sessionAt := time.Now()
			phase("capture")
			at = time.Now()
			readCtx, stop := context.WithTimeout(ctx, 250*time.Millisecond)
			o, e := extract(readCtx, source, extractOptions{Session: sid, MaxRows: 1000})
			stop()
			r.MaxAttemptNS = max(r.MaxAttemptNS, time.Since(at).Nanoseconds())
			var stage *sql.DB
			var stagePath string
			if errors.Is(e, errBounded) || errors.Is(e, context.DeadlineExceeded) {
				var path string
				stage, path, e = newStage()
				stagePath = path
				if e != nil {
					return r, e
				}
				if hooks.StageFile != nil {
					hooks.StageFile(path)
				}
				readCtx, stop = context.WithTimeout(ctx, 2*time.Second)
				attempt := time.Now()
				o, e = stageSession(readCtx, source, stage, extractOptions{Session: sid})
				stop()
				r.MaxAttemptNS = max(r.MaxAttemptNS, time.Since(attempt).Nanoseconds())
				r.Staged++
			}
			r.CaptureNS += time.Since(at).Nanoseconds()
			if e != nil {
				if stage != nil {
					_ = stage.Close()
					_ = os.Remove(stagePath)
					_ = os.Remove(stagePath + "-journal")
					_ = os.Remove(stagePath + "-wal")
					_ = os.Remove(stagePath + "-shm")
				}
				if ctx.Err() != nil {
					return r, ctx.Err()
				}
				if errors.Is(e, context.DeadlineExceeded) {
					r.Deferred++
					continue
				}
				return r, fmt.Errorf("capture %s: %w", sid, e)
			}
			var gen int64
			e = archive.QueryRowContext(ctx, `SELECT generation FROM accepted WHERE session=?`, sid).Scan(&gen)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return r, e
			}
			phase("prepublication")
			if hooks.BeforePublish != nil {
				hooks.BeforePublish()
			}
			at = time.Now()
			_, e = publishSession(ctx, archive, sid, o, stage, publishOptions{Expected: gen, BeforeWrite: func() { phase("publication") }})
			r.PublishNS += time.Since(at).Nanoseconds()
			if stage != nil {
				_ = stage.Close()
				_ = os.Remove(stagePath)
				_ = os.Remove(stagePath + "-journal")
				_ = os.Remove(stagePath + "-wal")
				_ = os.Remove(stagePath + "-shm")
			}
			if e != nil {
				return r, e
			}
			r.Sessions++
			r.MaxSessionNS = max(r.MaxSessionNS, time.Since(sessionAt).Nanoseconds())
			inBatch++
		}
		cursor = ids[len(ids)-1]
	}
	r.MaxBatchNS = max(r.MaxBatchNS, time.Since(batch).Nanoseconds())
	phase("idle")
	return r, nil
}
