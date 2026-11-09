package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/kimoofey/tui/ocm/history/diagnostics"
	"sync"

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

// Open the archive before doing any source discovery. Init's commands perform
// discovery/load while the program can already render saved history.
func loadHistoryBrowser(open func() (*history.Store, error), discover func(context.Context) (session.Source, error), launcher service.Launcher, load service.LoadOptions) (ocm.Options, error) {
	archive, err := open()
	if err != nil {
		return ocm.Options{}, err
	}
	browser := service.NewHistoryBrowser(archive, discover, launcher, load)
	return ocm.Options{History: browser, RootOnly: load.RootOnly, CostPeriod: load.CostPeriod, Close: browser.Close}, nil
}

// No diagnostic path or factory is evaluated unless both opt-ins are true.
func loadHistoryWithDiagnostics(config history.Config, openLog func() (*diagnostics.Logger, error), open func() (*history.Store, error), discover func(context.Context) (session.Source, error), launcher service.Launcher, load service.LoadOptions) (ocm.Options, error) {
	var log *diagnostics.Logger
	if config.History.Enabled && config.History.Diagnostics.Enabled {
		var err error
		log, err = openLog()
		if err != nil {
			log.Close()
			return ocm.Options{}, fmt.Errorf("opening history diagnostic log: %w", err)
		}
	}
	log.Record(diagnostics.Event{Kind: diagnostics.Startup, Stage: diagnostics.Begin})
	if log.Warning() != "" {
		log.Close()
		return ocm.Options{}, errors.New("initializing history diagnostic log: unable to write; check storage permissions and free space")
	}
	archive, err := open()
	if err != nil {
		log.Record(diagnostics.Event{Kind: diagnostics.Startup, Stage: diagnostics.Archive, Code: diagnostics.Failed})
		log.Close()
		return ocm.Options{}, err
	}
	browser := service.NewHistoryBrowser(archive, discover, launcher, load, log)
	log.Record(diagnostics.Event{Kind: diagnostics.Startup})
	if log.Warning() != "" {
		browser.Close()
		log.Close()
		return ocm.Options{}, errors.New("initializing history diagnostic log: unable to write; check storage permissions and free space")
	}
	var once sync.Once
	var closeErr error
	closeAll := func() error {
		once.Do(func() {
			err := browser.Close()
			code := diagnostics.OK
			if err != nil {
				code = diagnostics.Failed
			}
			log.Record(diagnostics.Event{Kind: diagnostics.Shutdown, Code: code})
			closeErr = errors.Join(err, log.Close())
		})
		return closeErr
	}
	return ocm.Options{History: browser, RootOnly: load.RootOnly, CostPeriod: load.CostPeriod, Close: closeAll}, nil
}
