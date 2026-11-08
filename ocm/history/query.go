package history

import (
	"context"
	"database/sql"
	"math"
)

// Range uses a half-open millisecond interval. Nil selects all time.
type Range struct{ Start, End int64 }
type Total struct {
	Cost              *float64
	Rows, UnknownCost int64
}
type Day struct {
	UTC string
	Total
}
type Model struct {
	Provider, Name *string
	Total
}
type SessionStatus struct {
	ID, Status    string
	Captured      int64
	ApproximateV1 bool
}
type Report struct {
	Total
	Days                                      []Day
	Models                                    []Model
	Sessions                                  []SessionStatus
	Unbucketed, UnknownProvider, UnknownModel int64
	Tokens                                    [5]*int64
	ApproximateV1                             bool
}

func readTotal(row interface{ Scan(...any) error }, t *Total) error {
	err := row.Scan(&t.Cost, &t.Rows, &t.UnknownCost)
	if t.Cost != nil && math.IsInf(*t.Cost, 0) {
		t.Cost = nil
	}
	return err
}

// Query reads every component from one committed SQLite snapshot.
func (s *Store) Query(ctx context.Context, period *Range) (Report, error) {
	var out Report
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	where := ""
	var args []any
	if period != nil {
		where = " WHERE timestamp>=? AND timestamp<?"
		args = []any{period.Start, period.End}
	}
	const totals = `SUM(cost),COUNT(*),COUNT(*)-COUNT(cost)`
	if err = readTotal(tx.QueryRowContext(ctx, `SELECT `+totals+` FROM usage`+where, args...), &out.Total); err != nil {
		return out, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(timestamp IS NULL),0),COALESCE(SUM(provider IS NULL),0),COALESCE(SUM(model IS NULL),0) FROM usage`+where, args...).Scan(&out.Unbucketed, &out.UnknownProvider, &out.UnknownModel); err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT strftime('%Y-%m-%d',timestamp/1000.0,'unixepoch'),`+totals+` FROM usage`+where+` GROUP BY 1 ORDER BY 1`, args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var d Day
		var date *string
		if err = rows.Scan(&date, &d.Cost, &d.Rows, &d.UnknownCost); err != nil {
			rows.Close()
			return out, err
		}
		if d.Cost != nil && math.IsInf(*d.Cost, 0) {
			d.Cost = nil
		}
		if date != nil {
			d.UTC = *date
			out.Days = append(out.Days, d)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT provider,model,`+totals+` FROM usage`+where+` GROUP BY provider,model ORDER BY provider,model`, args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var m Model
		if err = rows.Scan(&m.Provider, &m.Name, &m.Cost, &m.Rows, &m.UnknownCost); err != nil {
			rows.Close()
			return out, err
		}
		if m.Cost != nil && math.IsInf(*m.Cost, 0) {
			m.Cost = nil
		}
		out.Models = append(out.Models, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT input,output,reasoning,cache_read,cache_write FROM usage`+where, args...)
	if err != nil {
		return out, err
	}
	var sums [5]int64
	var seen, bad [5]bool
	for rows.Next() {
		var vals [5]*int64
		if err = rows.Scan(&vals[0], &vals[1], &vals[2], &vals[3], &vals[4]); err != nil {
			rows.Close()
			return out, err
		}
		for i, v := range vals {
			if v == nil {
				bad[i] = true
				continue
			}
			seen[i] = true
			if *v > math.MaxInt64-sums[i] {
				bad[i] = true
			} else {
				sums[i] += *v
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for i := range sums {
		if seen[i] && !bad[i] {
			v := sums[i]
			out.Tokens[i] = &v
		}
	}
	rows, err = tx.QueryContext(ctx, `SELECT id,status,captured,v1 FROM sessions ORDER BY id`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var status SessionStatus
		if err = rows.Scan(&status.ID, &status.Status, &status.Captured, &status.ApproximateV1); err != nil {
			rows.Close()
			return out, err
		}
		out.Sessions = append(out.Sessions, status)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	// Quality is specific to usage selected by this range, not unrelated sessions.
	qualityWhere := where
	if qualityWhere == "" {
		qualityWhere = " WHERE v1=1"
	} else {
		qualityWhere += " AND v1=1"
	}
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usage JOIN sessions ON sessions.id=usage.session_id`+qualityWhere+`)`, args...).Scan(&out.ApproximateV1)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
