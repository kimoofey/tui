//go:build !mock

package main

import (
	"fmt"

	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/db"
)

func loadData(sessionsOnly bool) (ocm.Options, error) {
	dbPath, err := db.DBPath()
	if err != nil {
		return ocm.Options{}, fmt.Errorf("resolving db path: %w", err)
	}

	sessions, err := db.LoadSessions(dbPath, sessionsOnly)
	if err != nil {
		return ocm.Options{}, fmt.Errorf("loading sessions from %s: %w", dbPath, err)
	}

	totalCount, err := db.SessionCount(dbPath)
	if err != nil {
		return ocm.Options{}, fmt.Errorf("counting sessions: %w", err)
	}

	dbUsed, dbTotal, err := db.DBStats(dbPath)
	if err != nil {
		return ocm.Options{}, fmt.Errorf("reading db stats: %w", err)
	}

	orphanCount, orphanBytes, _ := db.OrphanStats(dbPath)

	return ocm.Options{
		Sessions:    sessions,
		TotalCount:  totalCount,
		DBUsed:      dbUsed,
		DBTotal:     dbTotal,
		OrphanCount: orphanCount,
		OrphanBytes: orphanBytes,
		DBPath:      dbPath,
		RootOnly:    sessionsOnly,
	}, nil
}
