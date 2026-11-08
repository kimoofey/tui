package main

import (
	"context"
	"errors"

	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/history"
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

// Disabled mode never evaluates the archive factory.
func loadHistorySource(source session.Source, launcher service.Launcher, load service.LoadOptions, enabled bool, open func() (*history.Store, error)) (ocm.Options, error) {
	if !enabled {
		return loadSource(source, launcher, load)
	}
	archive, err := open()
	if err != nil {
		source.Close()
		return ocm.Options{}, err
	}
	app := service.New(source, launcher, nil)
	app.EnableHistory(archive, 0)
	closeAll := func() error { app.CloseHistory(); return errors.Join(source.Close(), archive.Close()) }
	snap, err := app.Load(context.Background(), load)
	if err != nil {
		closeAll()
		return ocm.Options{}, err
	}
	opts := ocm.OptionsFromSnapshot(snap, app, load)
	opts.Close = closeAll
	return opts, nil
}
