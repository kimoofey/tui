package main

import (
	"context"

	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

// The composition layer owns the source, including failed startup loads.
func loadSource(source session.Source, launcher service.Launcher, load service.LoadOptions) (ocm.Options, error) {
	app := service.New(source, launcher, nil)
	snap, err := app.Load(context.Background(), load)
	if err != nil {
		_ = source.Close()
		return ocm.Options{}, err
	}
	opts := ocm.OptionsFromSnapshot(snap, app, load)
	opts.Close = source.Close
	return opts, nil
}
