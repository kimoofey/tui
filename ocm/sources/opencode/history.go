package opencode

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"

	"github.com/kimoofey/tui/ocm/session"
)

// Every operation owns a dedicated read-only connection; browser queries never
// wait for a collector to release the browser handle.
func (s *Store) historyDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", sqliteURI(s.Path(), "ro"))
	if err == nil {
		db.SetMaxOpenConns(1)
	}
	return db, err
}
func historySchema(ctx context.Context, tx *sql.Tx) (string, error) {
	var table string
	err := tx.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name IN ('session_v2','session') ORDER BY CASE name WHEN 'session_v2' THEN 0 ELSE 1 END LIMIT 1`).Scan(&table)
	return table, err
}
func (s *Store) HistorySessions(ctx context.Context) ([]string, error) {
	db, err := s.historyDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	table, err := historySchema(ctx, tx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM `+table+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return ids, tx.Commit()
}
func (s *Store) CaptureHistory(ctx context.Context, id string) (_ session.Observation, err error) {
	var o session.Observation
	o.SessionID = id
	db, err := s.historyDB()
	if err != nil {
		return o, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return o, err
	}
	defer tx.Rollback()
	table, err := historySchema(ctx, tx)
	if err != nil {
		return o, err
	}
	path, err := filepath.EvalSymlinks(s.Path())
	if err != nil {
		return o, err
	}
	o.Binding = session.HistoryBinding{Path: path, Schema: table}
	o.ApproximateV1 = table == "session"
	var exists int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE id=?`, id).Scan(&exists); err != nil {
		return o, err
	}
	if exists > 1 {
		return o, errors.New("duplicate session identity")
	}
	o.Present = exists == 1
	projection := func(alias string) string {
		return `CASE WHEN json_valid(` + alias + `.data) THEN json_extract(` + alias + `.data,'$.role','$.source','$.cost','$.model','$.providerID','$.modelID','$.tokens') END`
	}
	q := `SELECT m.id,CASE WHEN typeof(m.time_created)='integer' THEN m.time_created END,'message',NULL,` + projection("m") + ` FROM message m WHERE m.session_id=?`
	args := []any{id}
	var fork *string
	var boundary any
	if table == "session_v2" {
		if o.Present {
			if err = tx.QueryRowContext(ctx, `SELECT fork_session_id,time_created FROM session_v2 WHERE id=?`, id).Scan(&fork, &boundary); err != nil {
				return o, err
			}
		}
		if fork != nil {
			if _, ok := boundary.(int64); !ok {
				return o, errors.New("unknown fork boundary")
			}
		}
		q = `SELECT m.id,CASE WHEN typeof(m.time_created)='integer' THEN m.time_created END,'message',m.type,` + projection("m") + ` FROM session_message m WHERE m.session_id=? UNION ALL SELECT e.id,CASE WHEN typeof(e.created)='integer' THEN e.created END,'event',e.type,` + projection("e") + ` FROM event e WHERE e.aggregate_id=? AND (e.type='session.usage.recorded.1' OR typeof(e.type)!='text')`
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return o, err
	}
	defer rows.Close()
	seen := map[[2]string]bool{}
	for rows.Next() {
		if !o.Present {
			continue
		}
		if err = ctx.Err(); err != nil {
			return session.Observation{}, err
		}
		var u session.Usage
		var discriminator, raw *string
		if err = rows.Scan(&u.ID, &u.Timestamp, &u.Kind, &discriminator, &raw); err != nil {
			return session.Observation{}, err
		}
		if raw == nil {
			return session.Observation{}, errors.New("malformed accounting eligibility")
		}
		var fields [7]json.RawMessage
		if err = json.Unmarshal([]byte(*raw), &fields); err != nil {
			return session.Observation{}, errors.New("malformed accounting projection")
		}
		if u.Kind == "message" {
			role := textValue(fields[0])
			if table == "session_v2" {
				role = discriminator
			}
			if role == nil {
				return session.Observation{}, errors.New("unknown message role")
			}
			switch *role {
			case "user", "system", "tool":
				continue
			case "assistant":
			default:
				return session.Observation{}, errors.New("unknown message role")
			}
			if fork != nil {
				if u.Timestamp == nil {
					return session.Observation{}, errors.New("unknown fork message timestamp")
				}
				if *u.Timestamp < boundary.(int64) {
					continue
				}
			}
		} else {
			if discriminator == nil || *discriminator != "session.usage.recorded.1" {
				return session.Observation{}, errors.New("unknown usage event type")
			}
			source := textValue(fields[1])
			if source == nil {
				return session.Observation{}, errors.New("unknown usage event source")
			}
			if *source != "compaction" {
				continue
			}
		}
		u.Cost = costValue(fields[2])
		u.Provider = textValue(fields[4])
		u.Model = textValue(fields[5])
		if table == "session_v2" {
			var model struct {
				Provider json.RawMessage `json:"providerID"`
				ID       json.RawMessage `json:"id"`
			}
			_ = json.Unmarshal(fields[3], &model)
			u.Provider = textValue(model.Provider)
			u.Model = textValue(model.ID)
		}
		var tokens struct {
			Input, Output, Reasoning json.RawMessage
			Cache                    struct{ Read, Write json.RawMessage }
		}
		_ = json.Unmarshal(fields[6], &tokens)
		u.Input = counterValue(tokens.Input)
		u.Output = counterValue(tokens.Output)
		u.Reasoning = counterValue(tokens.Reasoning)
		u.CacheRead = counterValue(tokens.Cache.Read)
		u.CacheWrite = counterValue(tokens.Cache.Write)
		key := [2]string{u.Kind, u.ID}
		if u.ID == "" || seen[key] {
			return session.Observation{}, errors.New("duplicate or empty usage identity")
		}
		seen[key] = true
		o.Usage = append(o.Usage, u)
	}
	if err = rows.Err(); err != nil {
		return session.Observation{}, err
	}
	if err = rows.Close(); err != nil {
		return session.Observation{}, err
	}
	if err = tx.Commit(); err != nil {
		return session.Observation{}, err
	}
	return o, nil
}
func textValue(raw json.RawMessage) *string {
	var s string
	if json.Unmarshal(raw, &s) != nil || s == "" {
		return nil
	}
	return &s
}
func costValue(raw json.RawMessage) *float64 {
	if string(raw) == "null" {
		return nil
	}
	var n float64
	if json.Unmarshal(raw, &n) != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
		return nil
	}
	return &n
}
func counterValue(raw json.RawMessage) *int64 {
	if string(raw) == "null" {
		return nil
	}
	var n int64
	if json.Unmarshal(raw, &n) != nil || n < 0 {
		return nil
	}
	return &n
}
