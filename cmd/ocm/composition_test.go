package main

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

type ownedSource struct {
	closed int
	fail   bool
	empty  bool
}

func (f *ownedSource) Info() session.Info { return session.Info{ID: "fake"} }
func (f *ownedSource) Close() error       { f.closed++; return nil }
func (f *ownedSource) List(context.Context, session.ListOptions) (session.Listing, error) {
	if f.fail {
		return session.Listing{}, errors.New("load failed")
	}
	if f.empty {
		return session.Listing{}, nil
	}
	return session.Listing{Sessions: []session.Session{{Ref: session.Ref{Source: "fake", ID: "one"}}}, TotalCount: 1}, nil
}
func TestSourceOwnership(t *testing.T) {
	for _, mode := range []string{"load failure", "empty", "normal", "program failure"} {
		t.Run(mode, func(t *testing.T) {
			source := &ownedSource{fail: mode == "load failure", empty: mode == "empty"}
			opts, err := loadSource(source, nil, service.LoadOptions{})
			if source.fail {
				if err == nil || source.closed != 1 {
					t.Fatal("load failure leaked source")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if source.closed != 0 {
				t.Fatal("loader closed live source")
			}
			called := false
			code := runBrowser(opts, func(tea.Model) error {
				called = true
				if mode == "program failure" {
					return errors.New("TUI failed")
				}
				return nil
			})
			if source.closed != 1 || called == source.empty || (code == 1) != (mode == "program failure") {
				t.Fatalf("cleanup: closed=%d program=%v code=%d", source.closed, called, code)
			}
		})
	}
}
