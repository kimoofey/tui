//go:build !mock

package main

import (
	"fmt"
	"time"

	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/db"
)

func loadData(sessionsOnly bool, costPeriod ocm.CostPeriod) (ocm.Options, error) {
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

	var periodCost float64
	if costPeriod != "" {
		start, end, err := ocm.CostPeriodBounds(costPeriod, time.Now())
		if err != nil {
			return ocm.Options{}, fmt.Errorf("computing period bounds (%s): %w", costPeriod, err)
		}
		periodCost, err = db.PeriodCost(dbPath, start.UnixMilli(), end.UnixMilli())
		if err != nil {
			return ocm.Options{}, fmt.Errorf("reading period cost (%s): %w", costPeriod, err)
		}
	}

	return ocm.Options{
		Sessions:    sessions,
		TotalCount:  totalCount,
		DBUsed:      dbUsed,
		DBTotal:     dbTotal,
		CostPeriod:  costPeriod,
		PeriodCost:  periodCost,
		OrphanCount: orphanCount,
		OrphanBytes: orphanBytes,
		DBPath:      dbPath,
		RootOnly:    sessionsOnly,
	}, nil
}
