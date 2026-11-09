//go:build !mock

package main

import (
	"context"
	"github.com/kimoofey/tui/internal/terminal"
	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/history/diagnostics"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
	"github.com/kimoofey/tui/ocm/sources/opencode"
)

func loadData(sessionsOnly bool, period service.CostPeriod) (ocm.Options, error) {
	configPath, err := history.ConfigPath()
	if err != nil {
		return ocm.Options{}, err
	}
	config, err := history.LoadConfig(configPath)
	if err != nil {
		return ocm.Options{}, err
	}

	launcher := terminal.Launcher{Terminal: ui.ResolveTerminal("")}
	load := service.LoadOptions{RootOnly: sessionsOnly, CostPeriod: period}
	if !config.History.Enabled {
		source, err := opencode.Discover()
		if err != nil {
			return ocm.Options{}, err
		}
		return loadSource(source, launcher, load)
	}
	return loadHistoryWithDiagnostics(config, func() (*diagnostics.Logger, error) {
		path, err := diagnostics.Path()
		if err != nil {
			return nil, err
		}
		return diagnostics.Open(path, version)
	}, func() (*history.Store, error) {
		path, err := history.ArchivePath()
		if err != nil {
			return nil, err
		}
		return history.Open(path)
	}, func(ctx context.Context) (session.Source, error) { return opencode.DiscoverContext(ctx) }, launcher, load)
}
