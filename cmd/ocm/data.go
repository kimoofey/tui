//go:build !mock

package main

import (
	"github.com/kimoofey/tui/internal/terminal"
	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/service"
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
	source, err := opencode.Discover()
	if err != nil {
		return ocm.Options{}, err
	}
	opts, err := loadHistorySource(source, terminal.Launcher{Terminal: ui.ResolveTerminal("")}, service.LoadOptions{RootOnly: sessionsOnly, CostPeriod: period}, config.History.Enabled, func() (*history.Store, error) {
		path, err := history.ArchivePath()
		if err != nil {
			return nil, err
		}
		return history.Open(path)
	})
	return opts, err
}
