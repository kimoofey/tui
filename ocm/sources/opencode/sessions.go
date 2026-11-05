package opencode

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type sessionRow struct {
	ID        string
	Title     string
	Directory string
	Created   time.Time
	Updated   time.Time
	Cost      float64
}

type Store struct {
	conn    *sql.DB
	storage storageContext
}

func Open(dbPath string) (*Store, error) {
	return openStorage(storageContext{DBPath: dbPath, DataDir: filepath.Dir(dbPath)})
}

func sqliteURI(path, mode string) string {
	return (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=" + mode}).String()
}

func openStorage(storage storageContext) (*Store, error) {
	if storage.DBPath == ":memory:" {
		return nil, errors.New("in-memory storage is unsupported; OCM requires a database file")
	}
	var err error
	storage.DBPath, err = filepath.Abs(storage.DBPath)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	storage.DataDir, err = filepath.Abs(storage.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
	info, err := os.Stat(storage.DBPath)
	if err != nil {
		return nil, fmt.Errorf("access database %s: %w", storage.DBPath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("database %s is not a regular file", storage.DBPath)
	}
	conn, err := sql.Open("sqlite", sqliteURI(storage.DBPath, "ro"))
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	conn.SetMaxOpenConns(1)
	store := &Store{conn: conn, storage: storage}
	if _, err := store.sessionTableContext(context.Background()); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read database %s: %w", storage.DBPath, err)
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.conn.Close()
}

func (s *Store) Path() string { return s.storage.DBPath }

// sessionTable probes each time so a migration during OCM's lifetime is seen.
// Never fall back from an empty or incompatible V2 table to frozen V1 rows.
func (s *Store) sessionTableContext(ctx context.Context) (string, error) {
	var table string
	err := s.conn.QueryRowContext(ctx, `SELECT name FROM sqlite_master
		WHERE type = 'table' AND name IN ('session_v2', 'session')
		ORDER BY CASE name WHEN 'session_v2' THEN 0 ELSE 1 END LIMIT 1`).Scan(&table)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("unsupported OpenCode schema: no session or session_v2 table")
	}
	if err != nil {
		return "", fmt.Errorf("detect session schema: %w", err)
	}
	// Table names come only from the fixed allowlist above.
	rows, err := s.conn.QueryContext(ctx, `SELECT id, title, directory, time_created, time_updated, cost, parent_id FROM `+table+` LIMIT 0`)
	if err != nil {
		return "", fmt.Errorf("unsupported OpenCode schema (%s): %w", table, err)
	}
	_ = rows.Close()
	return table, nil
}

// DBStats returns used and total byte sizes of the database file, derived
// from SQLite page metrics. used = (page_count - freelist_count) * page_size,
// total = page_count * page_size.
func (s *Store) dbStats(ctx context.Context) (used, total int64, err error) {
	var pageSize, pageCount, freelistCount int64
	if err := s.conn.QueryRowContext(ctx, `PRAGMA page_size`).Scan(&pageSize); err != nil {
		return 0, 0, fmt.Errorf("pragma page_size: %w", err)
	}
	if err := s.conn.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&pageCount); err != nil {
		return 0, 0, fmt.Errorf("pragma page_count: %w", err)
	}
	if err := s.conn.QueryRowContext(ctx, `PRAGMA freelist_count`).Scan(&freelistCount); err != nil {
		return 0, 0, fmt.Errorf("pragma freelist_count: %w", err)
	}

	total = pageSize * pageCount
	used = pageSize * (pageCount - freelistCount)
	return used, total, nil
}

// VacuumDB runs VACUUM on the database to reclaim free pages, then returns
// updated used and total byte sizes read from the shared connection.
func (s *Store) vacuumDB(ctx context.Context) (used, total int64, err error) {
	rw, err := sql.Open("sqlite", sqliteURI(s.Path(), "rw"))
	if err != nil {
		return 0, 0, fmt.Errorf("open db for vacuum: %w", err)
	}
	defer func() {
		if cerr := rw.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if _, err := rw.ExecContext(ctx, `VACUUM`); err != nil {
		return 0, 0, fmt.Errorf("vacuum: %w", err)
	}

	return s.dbStats(ctx)
}

// LoadSessions returns sessions sorted by last-updated descending.
// If rootOnly is true, only top-level sessions (parent_id IS NULL) are returned.
func (s *Store) loadSessions(ctx context.Context, rootOnly bool) ([]sessionRow, error) {
	table, err := s.sessionTableContext(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT id, COALESCE(title, 'Untitled session'), directory, time_created, time_updated, cost FROM ` + table
	if rootOnly {
		q += ` WHERE parent_id IS NULL`
	}
	q += ` ORDER BY time_updated DESC, id DESC`

	rows, err := s.conn.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []sessionRow
	for rows.Next() {
		var sess sessionRow
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

func (s *Store) diffDir() string {
	return filepath.Join(s.storage.DataDir, "storage", "session_diff")
}

// loadSessionIDs returns a set of all session IDs currently in the database.
func (s *Store) loadSessionIDsContext(ctx context.Context) (map[string]struct{}, error) {
	table, err := s.sessionTableContext(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.conn.QueryContext(ctx, `SELECT id FROM `+table)
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
func (s *Store) orphanStats(ctx context.Context) (count int, bytes int64, err error) {
	if !s.storage.PruneSupported() {
		return 0, 0, nil
	}
	ids, err := s.loadSessionIDsContext(ctx)
	if err != nil {
		return 0, 0, err
	}

	entries, err := os.ReadDir(s.diffDir())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("read diff dir: %w", err)
	}

	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return count, bytes, err
		}
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
func (s *Store) pruneOrphans(ctx context.Context) (count int, bytes int64, err error) {
	if !s.storage.PruneSupported() {
		return 0, 0, errors.New("pruning unavailable for custom/channel databases: legacy diff files are shared")
	}
	ids, err := s.loadSessionIDsContext(ctx)
	if err != nil {
		return 0, 0, err
	}

	dir := s.diffDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("read diff dir: %w", err)
	}

	var removeErrs []string
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return count, bytes, err
		}
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
func (s *Store) sessionCount(ctx context.Context) (int, error) {
	table, err := s.sessionTableContext(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	if err := s.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
		return 0, fmt.Errorf("count sessions: %w", err)
	}
	return count, nil
}

// PeriodCost returns summed message-level costs in [startMs, endMs).
func (s *Store) periodCost(ctx context.Context, startMs, endMs int64) (float64, error) {
	table, err := s.sessionTableContext(ctx)
	if err != nil {
		return 0, err
	}
	if table == "session_v2" {
		return s.v2PeriodCostContext(ctx, startMs, endMs)
	}
	q := `SELECT COALESCE(SUM(CAST(json_extract(m.data, '$.cost') AS REAL)), 0)
		FROM message m
		WHERE m.time_created >= ? AND m.time_created < ?`

	var total float64
	if err := s.conn.QueryRowContext(ctx, q, startMs, endMs).Scan(&total); err != nil {
		return 0, fmt.Errorf("sum period cost: %w", err)
	}
	return total, nil
}

// V2 stores assistant costs in projections and compaction costs in usage
// events. A fork copies old messages but must not count that spend twice.
// Match OpenCode v2.1 packages/core/src/session/stats.ts.
func (s *Store) v2PeriodCostContext(ctx context.Context, startMs, endMs int64) (float64, error) {
	const q = `SELECT COALESCE(SUM(cost), 0) FROM (
		SELECT CAST(json_extract(m.data, '$.cost') AS REAL) AS cost
		FROM session_message m JOIN session_v2 s ON s.id = m.session_id
		WHERE m.type = 'assistant' AND m.time_created >= ? AND m.time_created < ?
			AND (s.fork_session_id IS NULL OR m.time_created >= s.time_created)
		UNION ALL
		SELECT CAST(json_extract(e.data, '$.cost') AS REAL) AS cost
		FROM event e JOIN session_v2 s ON s.id = e.aggregate_id
		WHERE e.type = 'session.usage.recorded.1'
			AND json_extract(e.data, '$.source') = 'compaction'
			AND e.created >= ? AND e.created < ?
	)`
	var total float64
	if err := s.conn.QueryRowContext(ctx, q, startMs, endMs, startMs, endMs).Scan(&total); err != nil {
		return 0, fmt.Errorf("sum V2 period cost: %w", err)
	}
	return total, nil
}
