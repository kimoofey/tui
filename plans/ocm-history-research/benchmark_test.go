package research

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func benchDatabase(b *testing.B, path, mode string) *sql.DB {
	b.Helper()
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=" + mode}).String())
	if err != nil {
		b.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	b.Cleanup(func() { _ = db.Close() })
	return db
}
func benchFixture(b *testing.B, n, sessions int) *sql.DB {
	b.Helper()
	path := filepath.Join(b.TempDir(), "synthetic.db")
	db := benchDatabase(b, path, "rwc")
	if _, err := db.Exec(legacySchema); err != nil {
		b.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	for s := 0; s < sessions; s++ {
		if _, err = tx.Exec(`INSERT INTO session VALUES(?,'Synthetic','/synthetic',1000,3000,0,NULL)`, fmt.Sprintf("s%06d", s)); err != nil {
			b.Fatal(err)
		}
	}
	stmt, err := tx.Prepare(`INSERT INTO message VALUES(?,?,?,?,?)`)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = stmt.Close() }()
	for i := 0; i < n; i++ {
		// Content padding makes SQLite JSON scanning include non-accounting bytes;
		// it never reaches retained observations or benchmark output.
		data := strings.TrimSuffix(payload(false, fmt.Sprintf("p%d", i%2), fmt.Sprintf("m%d", i%8), "", .125), "}") + `,"content":"` + strings.Repeat("x", 1024) + `"}`
		at := int64(1767225600000) + int64(i%31)*86400000
		if _, err = stmt.Exec(fmt.Sprintf("m%09d", i), fmt.Sprintf("s%06d", i%sessions), at, at, data); err != nil {
			b.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		b.Fatal(err)
	}
	return benchDatabase(b, path, "ro")
}
func BenchmarkHistory(b *testing.B) {
	for _, shape := range []struct {
		name           string
		rows, sessions int
	}{{"10k-100sessions", 10000, 100}, {"100k-1000sessions", 100000, 1000}, {"100k-huge-session", 100000, 1}} {
		b.Run(shape.name, func(b *testing.B) {
			source := benchFixture(b, shape.rows, shape.sessions)
			o, err := extract(context.Background(), source, extractOptions{})
			if err != nil {
				b.Fatal(err)
			}
			if len(o.Items) != shape.rows || total(o) != float64(shape.rows)*.125 {
				b.Fatal("benchmark oracle failed")
			}
			b.Run("Extract", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := extract(context.Background(), source, extractOptions{}); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("Group", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_ = aggregate(o, time.UTC, nil, nil)
				}
			})
			dest := benchDatabase(b, filepath.Join(b.TempDir(), "candidate.db"), "rwc")
			if _, err = dest.Exec(candidateSchema); err != nil {
				b.Fatal(err)
			}
			b.Run("WriteCandidate", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := writeCandidate(context.Background(), dest, o); err != nil {
						b.Fatal(err)
					}
				}
			})
			if err = writeCandidate(context.Background(), dest, o); err != nil {
				b.Fatal(err)
			}
			b.Run("SavedDailyQuery", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					rows, err := dest.Query(`SELECT day,sum(cost),sum(unknown_cost),max(quality) FROM daily GROUP BY day ORDER BY day`)
					if err != nil {
						b.Fatal(err)
					}
					for rows.Next() {
						var day, quality string
						var cost float64
						var unknown int
						if err = rows.Scan(&day, &cost, &unknown, &quality); err != nil {
							b.Fatal(err)
						}
					}
					if err = rows.Err(); err != nil {
						b.Fatal(err)
					}
					_ = rows.Close()
				}
			})
			var pageSize, pages int64
			if err = dest.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
				b.Fatal(err)
			}
			if err = dest.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
				b.Fatal(err)
			}
			b.Logf("candidate bytes=%d source rows=%d sessions=%d JSON padding/row=1024 provider/model pairs=8 days=31; warm single-process runs", pageSize*pages, shape.rows, shape.sessions)
		})
	}
}
func BenchmarkHistoryParseAccountingJSON(b *testing.B) {
	data := []byte(payload(false, "p", "m", "", .125))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			b.Fatal(err)
		}
	}
}
