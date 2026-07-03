package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Session represents a single OpenCode session row.
type Session struct {
	ID        string
	Title     string
	Directory string
	Created   time.Time
	Updated   time.Time
	Cost      float64
}

type Store struct {
	conn   *sql.DB
	dbPath string
}

func Open(dbPath string) (*Store, error) {
	conn, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	conn.SetMaxOpenConns(1)
	return &Store{conn: conn, dbPath: dbPath}, nil
}

func (s *Store) Close() error {
	return s.conn.Close()
}

func (s *Store) Path() string { return s.dbPath }

// DBPath returns the default path to the OpenCode SQLite database.
func DBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db"), nil
}

// DBStats returns used and total byte sizes of the database file, derived
// from SQLite page metrics. used = (page_count - freelist_count) * page_size,
// total = page_count * page_size.
func (s *Store) DBStats() (used, total int64, err error) {
	var pageSize, pageCount, freelistCount int64
	if err := s.conn.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		return 0, 0, fmt.Errorf("pragma page_size: %w", err)
	}
	if err := s.conn.QueryRow(`PRAGMA page_count`).Scan(&pageCount); err != nil {
		return 0, 0, fmt.Errorf("pragma page_count: %w", err)
	}
	if err := s.conn.QueryRow(`PRAGMA freelist_count`).Scan(&freelistCount); err != nil {
		return 0, 0, fmt.Errorf("pragma freelist_count: %w", err)
	}

	total = pageSize * pageCount
	used = pageSize * (pageCount - freelistCount)
	return used, total, nil
}

// VacuumDB runs VACUUM on the database to reclaim free pages, then returns
// updated used and total byte sizes read from the shared connection.
func (s *Store) VacuumDB() (used, total int64, err error) {
	rw, err := sql.Open("sqlite", "file:"+s.dbPath+"?mode=rw")
	if err != nil {
		return 0, 0, fmt.Errorf("open db for vacuum: %w", err)
	}
	defer func() {
		if cerr := rw.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if _, err := rw.Exec(`VACUUM`); err != nil {
		return 0, 0, fmt.Errorf("vacuum: %w", err)
	}

	return s.DBStats()
}

// LoadSessions returns sessions sorted by last-updated descending.
// If rootOnly is true, only top-level sessions (parent_id IS NULL) are returned.
func (s *Store) LoadSessions(rootOnly bool) ([]Session, error) {
	q := `SELECT id, title, directory, time_created, time_updated, cost
		FROM session
		ORDER BY time_updated DESC`
	if rootOnly {
		q = `SELECT id, title, directory, time_created, time_updated, cost
		FROM session
		WHERE parent_id IS NULL
		ORDER BY time_updated DESC`
	}

	rows, err := s.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []Session
	for rows.Next() {
		var sess Session
		var createdMs, updatedMs int64

		if err := rows.Scan(&sess.ID, &sess.Title, &sess.Directory, &createdMs, &updatedMs, &sess.Cost); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		sess.Created = time.UnixMilli(createdMs)
		sess.Updated = time.UnixMilli(updatedMs)
		sessions = append(sessions, sess)
	}

	return sessions, rows.Err()
}

// diffDir returns the path to the session_diff storage directory.
func diffDir(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), "storage", "session_diff")
}

// loadSessionIDs returns a set of all session IDs currently in the database.
func (s *Store) loadSessionIDs() (map[string]struct{}, error) {
	rows, err := s.conn.Query(`SELECT id FROM session`)
	if err != nil {
		return nil, fmt.Errorf("query session ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan id: %w", err)
		}
		ids[id] = struct{}{}
	}
	return ids, rows.Err()
}

// OrphanStats returns the count and total byte size of session_diff JSON files
// whose session no longer exists in the database.
func (s *Store) OrphanStats() (count int, bytes int64, err error) {
	ids, err := s.loadSessionIDs()
	if err != nil {
		return 0, 0, err
	}

	entries, err := os.ReadDir(diffDir(s.dbPath))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("read diff dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		sessionID := strings.TrimSuffix(name, ".json")
		if _, ok := ids[sessionID]; ok {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		count++
		bytes += info.Size()
	}
	return count, bytes, nil
}

// PruneOrphans deletes session_diff JSON files whose session no longer exists
// in the database. Returns the number of files deleted and bytes freed.
func (s *Store) PruneOrphans() (count int, bytes int64, err error) {
	ids, err := s.loadSessionIDs()
	if err != nil {
		return 0, 0, err
	}

	dir := diffDir(s.dbPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("read diff dir: %w", err)
	}

	var removeErrs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		sessionID := strings.TrimSuffix(name, ".json")
		if _, ok := ids[sessionID]; ok {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		size := info.Size()
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			removeErrs = append(removeErrs, name+": "+err.Error())
			continue
		}
		count++
		bytes += size
	}
	if len(removeErrs) > 0 {
		err = errors.New("failed to remove: " + strings.Join(removeErrs, "; "))
	}
	return count, bytes, err
}

// SessionCount returns the total number of sessions in the database,
// regardless of any filter applied to LoadSessions.
func (s *Store) SessionCount() (int, error) {
	var count int
	if err := s.conn.QueryRow(`SELECT COUNT(*) FROM session`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count sessions: %w", err)
	}
	return count, nil
}

// PeriodCost returns summed message-level costs in [startMs, endMs).
func (s *Store) PeriodCost(startMs, endMs int64) (float64, error) {
	q := `SELECT COALESCE(SUM(CAST(json_extract(m.data, '$.cost') AS REAL)), 0)
		FROM message m
		WHERE m.time_created >= ? AND m.time_created < ?`

	var total float64
	if err := s.conn.QueryRow(q, startMs, endMs).Scan(&total); err != nil {
		return 0, fmt.Errorf("sum period cost: %w", err)
	}
	return total, nil
}
