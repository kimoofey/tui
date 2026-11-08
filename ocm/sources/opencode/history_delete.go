package opencode

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kimoofey/tui/ocm/session"
)

// V1's CLI recursively filters children by its current project, which is not
// described by the source database binding. Refuse to guess that native scope.
func (s *Store) HistoryDeletionAvailability(ctx context.Context) session.Availability {
	db, err := s.historyDB()
	if err != nil {
		return session.Availability{Reason: err.Error()}
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return session.Availability{Reason: err.Error()}
	}
	defer tx.Rollback()
	table, err := historySchema(ctx, tx)
	if err != nil {
		return session.Availability{Reason: err.Error()}
	}
	if table != "session_v2" {
		return session.Availability{Reason: "history guarded deletion unsupported for V1: native children depend on the CLI project"}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,parent_id FROM session_v2 LIMIT 0`)
	if err != nil {
		return session.Availability{Reason: "unsupported history deletion layout: " + err.Error()}
	}
	rows.Close()
	return session.Availability{Available: true}
}
func (s *Store) HistoryDeletionScope(ctx context.Context, ref session.Ref) ([]session.Ref, error) {
	if ref.Source != s.Info().ID {
		return nil, session.ErrForeignSource
	}
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
	if table != "session_v2" {
		return nil, fmt.Errorf("%w: guarded deletion requires supported V2 recursive scope", session.ErrUnsupported)
	}
	var found int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM session_v2 WHERE id=?`, ref.ID).Scan(&found); err != nil {
		return nil, err
	}
	if found != 1 {
		return nil, errors.New("deletion target absent or duplicated")
	}
	// UNION deduplicates a malformed cycle instead of recursing indefinitely.
	rows, err := tx.QueryContext(ctx, `WITH RECURSIVE scope(id) AS (SELECT id FROM session_v2 WHERE id=? UNION SELECT s.id FROM session_v2 s JOIN scope p ON s.parent_id=p.id) SELECT s.id,s.parent_id FROM session_v2 s JOIN scope p ON s.id=p.id ORDER BY s.id`, ref.ID)
	if err != nil {
		return nil, err
	}
	parents := map[string]*string{}
	var refs []session.Ref
	for rows.Next() {
		var id string
		var parent *string
		if err = rows.Scan(&id, &parent); err != nil {
			rows.Close()
			return nil, err
		}
		parents[id] = parent
		refs = append(refs, session.Ref{Source: ref.Source, ID: id})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, r := range refs {
		seen := map[string]bool{}
		for id := r.ID; id != ""; {
			if seen[id] {
				return nil, fmt.Errorf("%w: cyclic native deletion scope", session.ErrUnsupported)
			}
			seen[id] = true
			p, ok := parents[id]
			if !ok || p == nil {
				break
			}
			id = *p
		}
	}
	return refs, tx.Commit()
}
