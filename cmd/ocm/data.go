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

	store, err := db.Open(dbPath)
	if err != nil {
		return ocm.Options{}, fmt.Errorf("opening database %s: %w", dbPath, err)
	}

	sessions, err := store.LoadSessions(sessionsOnly)
	if err != nil {
		return ocm.Options{}, fmt.Errorf("loading sessions from %s: %w", dbPath, err)
	}

	totalCount, err := store.SessionCount()
	if err != nil {
		return ocm.Options{}, fmt.Errorf("counting sessions: %w", err)
	}

	dbUsed, dbTotal, err := store.DBStats()
	if err != nil {
		return ocm.Options{}, fmt.Errorf("reading db stats: %w", err)
	}

	orphanCount, orphanBytes, _ := store.OrphanStats()

	var periodCost float64
	if costPeriod != "" {
		start, end, err := ocm.CostPeriodBounds(costPeriod, time.Now())
		if err != nil {
			return ocm.Options{}, fmt.Errorf("computing period bounds (%s): %w", costPeriod, err)
		}
		periodCost, err = store.PeriodCost(start.UnixMilli(), end.UnixMilli())
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
		Store:       store,
		RootOnly:    sessionsOnly,
	}, nil
}
