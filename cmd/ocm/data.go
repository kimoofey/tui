//go:build !mock

package main

import (
	"github.com/kimoofey/tui/internal/terminal"
	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/sources/opencode"
)

func loadData(sessionsOnly bool, period service.CostPeriod) (ocm.Options, error) {
	source, err := opencode.Discover()
	if err != nil {
		return ocm.Options{}, err
	}
	opts, err := loadSource(source, terminal.Launcher{Terminal: ui.ResolveTerminal("")}, service.LoadOptions{RootOnly: sessionsOnly, CostPeriod: period})
	return opts, err
}
