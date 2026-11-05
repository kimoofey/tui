package ocm

import (
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

func OptionsFromSnapshot(snap *service.Snapshot, app *service.Service, load service.LoadOptions) Options {
	opts := Options{Sessions: snap.Sessions, TotalCount: snap.TotalCount, Service: app, RootOnly: load.RootOnly, CostPeriod: load.CostPeriod, Capabilities: snap.Capabilities, MetricsKnown: snap.Metrics != nil, Diagnostics: snap.Diagnostics}
	if snap.Metrics != nil {
		opts.DBUsed, opts.DBTotal = snap.Metrics.Used, snap.Metrics.Total
	}
	if snap.PeriodCost != nil {
		opts.PeriodCost = snap.PeriodCost.Amount
	}
	if snap.Orphans != nil {
		opts.OrphanCount, opts.OrphanBytes = snap.Orphans.Count, snap.Orphans.Bytes
	}
	return opts
}
func formatRecordedCost(cost *session.RecordedCost) string {
	if cost == nil {
		return "n/a"
	}
	return formatCost(cost.Amount)
}
