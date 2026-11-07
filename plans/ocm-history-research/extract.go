package research

// This file is a research implementation, not a public adapter contract.
import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

const v1Policy = "v1-recorded-assistant-approximate/1"
const v2Policy = "v2-assistant-compaction/1"
const v1Warning = "Approximate: V1 forked sessions may include copied usage."

// Usage keeps accounting fields only; no source JSON or conversation content.
// Native IDs are namespaced by session and kind. Lineage is NOT inferred here.
type Usage struct {
	Session, Kind, ID, Provider, Model, Variant string
	At                                          int64
	TimeKnown                                   bool
	Cost                                        *float64
	Tokens                                      [5]*int64 // input, output, reasoning, cache read, cache write; never summed
}
type Observation struct {
	Schema, Policy, Quality, Scope string
	Complete                       bool            // scan scope only, never a billing-coverage assertion
	Sessions                       map[string]bool // includes known empty sessions; absent != empty
	Items                          map[string]Usage
	Diagnostics                    map[string]int
}

// Keep projected fields raw until validated. A typed envelope avoids allocating
// a map and keys per row without coercing malformed values into recorded zero.
type accountingFields struct {
	Role, Source, Cost, Model, ProviderID, ModelID, Variant, Tokens json.RawMessage
}

func stringValue(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

type extractOptions struct {
	Session       string // empty selects all sessions for full-import measurements
	AfterSnapshot func()
	AfterItem     func(int)
	MaxRows       int               // diagnostic experiment: a bounded read is never complete
	Sink          func(Usage) error // isolated disk-stage experiment; Items stays empty and Complete stays false
}

var errBounded = errors.New("bounded extraction is not a full snapshot")

func usageKey(u Usage) string { return u.Session + "\x00" + u.Kind + "\x00" + u.ID }

// extract owns no database lifecycle; caller supplies a dedicated mode=ro handle.
// Schema detection, candidate enumeration, and all usage reads share one snapshot.
// Errors return no publishable observation, including context/iteration errors.
func extract(ctx context.Context, db *sql.DB, opts extractOptions) (_ *Observation, err error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	o := &Observation{Sessions: map[string]bool{}, Items: map[string]Usage{}, Diagnostics: map[string]int{}}
	o.Scope = "all-sessions"
	if opts.Session != "" {
		o.Scope = "session:" + opts.Session
	}
	err = tx.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table'
 AND name IN ('session_v2','session') ORDER BY CASE name WHEN 'session_v2' THEN 0 ELSE 1 END LIMIT 1`).Scan(&o.Schema)
	if err != nil {
		return nil, fmt.Errorf("unsupported source: %w", err)
	}
	o.Policy = v2Policy
	if o.Schema == "session" {
		o.Policy = v1Policy
		o.Quality = v1Warning
	}
	// Detect missing required fields even when every table is empty. SQL names are
	// fixed, never supplied by a source. V2 cannot fall back to legacy tables.
	probes := []string{`SELECT id,time_created FROM ` + o.Schema + ` LIMIT 0`}
	if o.Schema == "session" {
		probes = append(probes, `SELECT id,session_id,time_created,data FROM message LIMIT 0`)
	} else {
		probes = append(probes, `SELECT fork_session_id FROM session_v2 LIMIT 0`,
			`SELECT id,session_id,type,time_created,data FROM session_message LIMIT 0`,
			`SELECT id,aggregate_id,type,created,data FROM event LIMIT 0`)
	}
	for _, q := range probes {
		rows, e := tx.QueryContext(ctx, q)
		if e != nil {
			return nil, fmt.Errorf("unsupported usage schema: %w", e)
		}
		if e = rows.Close(); e != nil {
			return nil, e
		}
	}
	if o.Schema == "session_v2" {
		var invalid int
		q := `SELECT count(*) FROM session_v2 WHERE fork_session_id IS NOT NULL AND typeof(time_created)!='integer'`
		var args []any
		if opts.Session != "" {
			q += ` AND id=?`
			args = append(args, opts.Session)
		}
		if err = tx.QueryRowContext(ctx, q, args...).Scan(&invalid); err != nil {
			return nil, err
		}
		if invalid != 0 {
			return nil, errors.New("fork boundary timestamp unavailable")
		}
	}
	candidates := `SELECT id FROM ` + o.Schema
	var args []any
	if opts.Session != "" {
		candidates += ` WHERE id=?`
		args = append(args, opts.Session)
	}
	rows, err := tx.QueryContext(ctx, candidates+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		o.Sessions[id] = true
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if opts.AfterSnapshot != nil {
		opts.AfterSnapshot()
	}
	q, args := usageQuery(o.Schema, opts.Session)
	rows, err = tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	n := 0
	for rows.Next() {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		var u Usage
		var stamp sql.NullInt64
		var raw sql.NullString
		if err = rows.Scan(&u.ID, &u.Session, &stamp, &u.Kind, &raw); err != nil {
			return nil, err
		}
		n++
		if opts.AfterItem != nil {
			opts.AfterItem(n)
		}
		if opts.MaxRows > 0 && n > opts.MaxRows {
			return nil, errBounded
		}
		if !raw.Valid {
			o.Diagnostics["malformed_json"]++
			continue
		}
		// The SQL projection has a fixed eight-slot envelope. Preserve each value
		// as raw JSON so cost/token types and integer boundaries stay strict.
		var projected [8]json.RawMessage
		if err = json.Unmarshal([]byte(raw.String), &projected); err != nil {
			return nil, err
		}
		fields := accountingFields{projected[0], projected[1], projected[2], projected[3], projected[4], projected[5], projected[6], projected[7]}

		if o.Schema == "session" && stringValue(fields.Role) != "assistant" {
			if stringValue(fields.Role) != "user" {
				o.Diagnostics["unknown_role"]++
			}
			continue
		}
		if u.Kind == "event" && stringValue(fields.Source) != "compaction" {
			if stringValue(fields.Source) == "" {
				o.Diagnostics["unknown_event_source"]++
			}
			continue
		}
		if !stamp.Valid {
			o.Diagnostics["missing_timestamp"]++
		}
		u.At = stamp.Int64
		u.TimeKnown = stamp.Valid
		u.Cost = number(fields.Cost)
		if u.Cost == nil {
			o.Diagnostics["unknown_cost"]++
		}
		if o.Schema == "session" {
			u.Provider = stringValue(fields.ProviderID)
			u.Model = stringValue(fields.ModelID)
			u.Variant = stringValue(fields.Variant)
		} else {
			var model struct {
				Provider string `json:"providerID"`
				ID       string `json:"id"`
				Variant  string `json:"variant"`
			}
			if json.Unmarshal(fields.Model, &model) == nil {
				u.Provider = model.Provider
				u.Model = model.ID
				u.Variant = model.Variant
			}
		}
		if u.Provider == "" || u.Model == "" {
			o.Diagnostics["unknown_model"]++
		}
		var tokens struct {
			Input, Output, Reasoning json.RawMessage
			Cache                    struct{ Read, Write json.RawMessage }
		}
		if e := json.Unmarshal(fields.Tokens, &tokens); e != nil {
			o.Diagnostics["invalid_tokens"]++
		}
		for i, v := range []json.RawMessage{tokens.Input, tokens.Output, tokens.Reasoning, tokens.Cache.Read, tokens.Cache.Write} {
			value := integer(v)
			u.Tokens[i] = value
			if value == nil {
				o.Diagnostics["unknown_token_counter"]++
			}
		}
		key := usageKey(u)
		if _, exists := o.Items[key]; exists {
			return nil, errors.New("duplicate native usage identity")
		}
		if opts.Sink != nil {
			if err = opts.Sink(u); err != nil {
				return nil, err
			}
		} else {
			o.Items[key] = u
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	// A streamed observation is metadata only, never publishable by writeCandidate.
	o.Complete = opts.Sink == nil
	return o, nil
}
func number(raw json.RawMessage) *float64 {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var n float64
	if json.Unmarshal(raw, &n) != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
		return nil
	}
	return &n
}
func integer(raw json.RawMessage) *int64 {
	// Strict int64 decode avoids rounding/overflow at the float64 boundary.
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var n int64
	if json.Unmarshal(raw, &n) != nil || n < 0 {
		return nil
	}
	return &n
}

type bucketKey struct{ Day, Provider, Model, Variant string }
type bucket struct {
	Cost                          float64
	KnownCost, UnknownCost, Count int
	Tokens                        [5]int64
	UnknownTokens                 [5]int
	Quality                       string
}

func aggregate(o *Observation, zone *time.Location, from, to *int64) map[bucketKey]bucket {
	out := map[bucketKey]bucket{}
	for _, u := range o.Items {
		// Unknown timestamps have an explicit unbucketed category for all-time
		// accounting; they cannot be assigned to an arbitrary day or date range.
		if !u.TimeKnown && (from != nil || to != nil) {
			continue
		}
		if from != nil && u.At < *from || to != nil && u.At >= *to {
			continue
		}
		day := ""
		if u.TimeKnown {
			day = time.UnixMilli(u.At).In(zone).Format("2006-01-02")
		}
		k := bucketKey{day, u.Provider, u.Model, u.Variant}
		b := out[k]
		b.Count++
		b.Quality = o.Quality
		if u.Cost == nil {
			b.UnknownCost++
		} else {
			b.KnownCost++
			b.Cost += *u.Cost
		}
		for i, n := range u.Tokens {
			if n == nil {
				b.UnknownTokens[i]++
			} else {
				b.Tokens[i] += *n
			}
		}
		out[k] = b
	}
	return out
}

// usageQuery projects accounting only; callers own one snapshot for every row.
func usageQuery(schema, session string) (string, []any) {
	// SQLite parses JSON inside the engine but returns only accounting fields.
	// CASE guards malformed JSON. Parse cost/token types in Go; never CAST strings
	// to zero or coalesce unknowns into recorded zero.
	projection := func(alias string) string {
		// A single multi-path extraction avoids repeatedly loading overflow
		// pages for the same large source JSON. Multiple paths return a JSON
		// array, retaining JSON types rather than SQL scalar coercions.
		return `CASE WHEN json_valid(` + alias + `.data) THEN json_extract(` + alias + `.data,
 '$.role','$.source','$.cost','$.model','$.providerID','$.modelID','$.variant','$.tokens') ELSE NULL END`
	}

	// No consumer depends on scan order: observations are identity maps and disk
	// stages enforce identity with their primary key. Sorting projected JSON before
	// the first row prevents early row-limit cancellation and wastes snapshot time.
	messageFilter, eventFilter := "", ""
	var args []any
	if session != "" {
		messageFilter = " AND m.session_id=?"
		eventFilter = " AND e.aggregate_id=?"
		args = append(args, session)
	}
	q := `SELECT m.id,m.session_id,CASE WHEN typeof(m.time_created)='integer' THEN m.time_created END,'message',` + projection("m") + `
 FROM message m JOIN session s ON s.id=m.session_id WHERE 1=1` + messageFilter
	if schema == "session_v2" {
		q = `SELECT m.id,m.session_id,CASE WHEN typeof(m.time_created)='integer' THEN m.time_created END,'message',` + projection("m") + `
 FROM session_message m JOIN session_v2 s ON s.id=m.session_id
 WHERE m.type='assistant' AND (s.fork_session_id IS NULL OR m.time_created>=s.time_created OR m.time_created IS NULL)` + messageFilter + `
 UNION ALL SELECT e.id,e.aggregate_id,CASE WHEN typeof(e.created)='integer' THEN e.created END,'event',` + projection("e") + `
 FROM event e JOIN session_v2 s ON s.id=e.aggregate_id WHERE e.type='session.usage.recorded.1'` + eventFilter
		if session != "" {
			args = append(args, session)
		}
	}
	return q, args
}
