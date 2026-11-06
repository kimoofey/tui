package research

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Candidate schema for capacity measurements only. This is NOT a migration or
// approved archive layout. Caller must supply a separate synthetic destination.
const candidateSchema = `CREATE TABLE usage (
 session TEXT,kind TEXT,id TEXT,at INTEGER,time_known INTEGER,
 provider TEXT,model TEXT,variant TEXT,cost REAL,tokens TEXT,
 policy TEXT,quality TEXT,PRIMARY KEY(session,kind,id)
) WITHOUT ROWID;
CREATE TABLE daily (
 day TEXT,provider TEXT,model TEXT,variant TEXT,cost REAL,known_cost INTEGER,
 unknown_cost INTEGER,steps INTEGER,quality TEXT,
 PRIMARY KEY(day,provider,model,variant)
) WITHOUT ROWID;
CREATE TABLE observation (id INTEGER PRIMARY KEY CHECK(id=1),schema_name TEXT,policy TEXT,
 quality TEXT,complete INTEGER,diagnostics TEXT);`

func writeCandidate(ctx context.Context, destination *sql.DB, o *Observation) error {
	if o == nil || !o.Complete {
		return errors.New("cannot publish incomplete observation")
	}
	if o.Scope != "all-sessions" {
		return errors.New("candidate writer only supports whole-fixture replacement")
	}
	tx, err := destination.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, table := range []string{"usage", "daily", "observation"} {
		if _, err = tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return err
		}
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO usage VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, u := range o.Items {
		tokens, e := json.Marshal(u.Tokens)
		if e != nil {
			return e
		}
		if _, err = stmt.ExecContext(ctx, u.Session, u.Kind, u.ID, u.At, u.TimeKnown, u.Provider, u.Model, u.Variant, u.Cost, string(tokens), o.Policy, o.Quality); err != nil {
			return err
		}
	}
	daily, err := tx.PrepareContext(ctx, `INSERT INTO daily VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer func() { _ = daily.Close() }()
	for k, b := range aggregate(o, time.UTC, nil, nil) {
		if _, err = daily.ExecContext(ctx, k.Day, k.Provider, k.Model, k.Variant, b.Cost, b.KnownCost, b.UnknownCost, b.Count, b.Quality); err != nil {
			return err
		}
	}
	diagnostics, err := json.Marshal(o.Diagnostics)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO observation VALUES(1,?,?,?,?,?)`, o.Schema, o.Policy, o.Quality, o.Complete, string(diagnostics)); err != nil {
		return err
	}
	return tx.Commit()
}
