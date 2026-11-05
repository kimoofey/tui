package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kimoofey/tui/internal/terminal"
	"github.com/kimoofey/tui/ocm/session"
)

type listSource struct {
	id      session.SourceID
	listing session.Listing
	listErr error
	lists   int
}

func (f *listSource) Info() session.Info { return session.Info{ID: f.id, Kind: "fake", Name: "Fake"} }
func (f *listSource) Close() error       { return nil }
func (f *listSource) List(ctx context.Context, _ session.ListOptions) (session.Listing, error) {
	f.lists++
	if err := ctx.Err(); err != nil {
		return session.Listing{}, err
	}
	return f.listing, f.listErr
}

type capableSource struct {
	listSource
	deletes                         []string
	resumes                         []string
	metricErr, periodErr, orphanErr error
	deleteErr                       error
	refreshErr                      error
	start, end                      time.Time
	deadlines                       []time.Duration
	vacuum, prune                   int
}

func (f *capableSource) Info() session.Info {
	info := f.listSource.Info()
	info.RootFilter = true
	return info
}
func (f *capableSource) Resume(ctx context.Context, id string) (terminal.Command, error) {
	f.resumes = append(f.resumes, id)
	return terminal.Command{Executable: "agent", Args: []string{"resume", id}}, nil
}
func (f *capableSource) Delete(ctx context.Context, id string) error {
	f.deletes = append(f.deletes, id)
	deadline, _ := ctx.Deadline()
	f.deadlines = append(f.deadlines, time.Until(deadline))
	if f.refreshErr != nil {
		f.listErr = f.refreshErr
	}
	f.listing = session.Listing{}
	return f.deleteErr
}
func (f *capableSource) Metrics(context.Context) (session.Metrics, error) {
	return session.Metrics{Used: 10, Total: 20}, f.metricErr
}
func (f *capableSource) PeriodCost(_ context.Context, start, end time.Time) (session.RecordedCost, error) {
	f.start, f.end = start, end
	return session.RecordedCost{Currency: "USD", Provenance: "source-recorded"}, f.periodErr
}
func (f *capableSource) VacuumAvailability() session.Availability {
	return session.Availability{Available: true}
}
func (f *capableSource) Vacuum(context.Context) (session.Metrics, error) {
	f.vacuum++
	return session.Metrics{}, nil
}
func (f *capableSource) OrphanAvailability() session.Availability {
	return session.Availability{Available: true}
}
func (f *capableSource) InspectOrphans(context.Context) (session.Orphans, error) {
	return session.Orphans{Count: 3}, f.orphanErr
}
func (f *capableSource) Prune(context.Context) (session.Orphans, error) {
	f.prune++
	return session.Orphans{}, nil
}

type capture struct{ commands []terminal.Command }

func (c *capture) Launch(cmd terminal.Command) error {
	c.commands = append(c.commands, cmd)
	return nil
}

func TestListOnlySource(t *testing.T) {
	source := &listSource{id: "list", listing: session.Listing{Sessions: []session.Session{{Ref: session.Ref{Source: "list", ID: "same"}}}, TotalCount: 1}}
	app := New(source, nil, nil)
	snap, err := app.Load(context.Background(), LoadOptions{})
	if err != nil || snap.Metrics != nil || snap.PeriodCost != nil || snap.Sessions[0].Cost != nil || snap.Capabilities.Resume || snap.Capabilities.Delete || snap.Capabilities.Vacuum.Available {
		t.Fatalf("list-only: %+v %v", snap, err)
	}
	for _, opts := range []LoadOptions{{RootOnly: true}, {CostPeriod: "week"}} {
		if snap, err := app.Load(context.Background(), opts); snap != nil || !errors.Is(err, session.ErrUnsupported) {
			t.Fatalf("unsupported load: %v %v", snap, err)
		}
	}
	ref := session.Ref{Source: "list", ID: "same"}
	if err := app.Resume(context.Background(), ref); !errors.Is(err, session.ErrUnsupported) {
		t.Fatal(err)
	}
	if result := app.Delete(context.Background(), []session.Ref{ref}, LoadOptions{}); !errors.Is(result.Err, session.ErrUnsupported) {
		t.Fatal(result.Err)
	}
	if _, err := app.Vacuum(context.Background()); !errors.Is(err, session.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := app.Prune(context.Background()); !errors.Is(err, session.ErrUnsupported) {
		t.Fatal(err)
	}
	if source.lists != 1 {
		t.Fatalf("unsupported requests listed source %d times", source.lists)
	}
}
func TestForeignReferencesNeverAct(t *testing.T) {
	a := &capableSource{listSource: listSource{id: "a"}}
	b := &capableSource{listSource: listSource{id: "b"}}
	launch := &capture{}
	apps := []*Service{New(a, launch, nil), New(b, launch, nil)}
	for i, app := range apps {
		own := session.Ref{Source: []session.SourceID{"a", "b"}[i], ID: "same"}
		foreign := session.Ref{Source: []session.SourceID{"b", "a"}[i], ID: "same"}
		if err := app.Resume(context.Background(), foreign); !errors.Is(err, session.ErrForeignSource) {
			t.Fatal(err)
		}
		result := app.Delete(context.Background(), []session.Ref{own, foreign}, LoadOptions{})
		if !errors.Is(result.Err, session.ErrForeignSource) {
			t.Fatal(result.Err)
		}
	}
	if len(a.deletes)+len(b.deletes)+len(a.resumes)+len(b.resumes)+len(launch.commands) != 0 || a.lists+b.lists != 0 {
		t.Fatal("foreign ref caused side effects")
	}
	if err := apps[0].Resume(context.Background(), session.Ref{Source: "a", ID: "same"}); err != nil {
		t.Fatal(err)
	}
	if err := apps[1].Resume(context.Background(), session.Ref{Source: "b", ID: "same"}); err != nil {
		t.Fatal(err)
	}
	if len(launch.commands) != 2 {
		t.Fatal("same-source resume not launched")
	}
}
func TestLoadErrorPolicyAndClock(t *testing.T) {
	boom := errors.New("broken optional data")
	for _, field := range []string{"listing", "metrics", "period", "orphans", "none"} {
		t.Run(field, func(t *testing.T) {
			source := &capableSource{listSource: listSource{id: "a"}}
			switch field {
			case "listing":
				source.listErr = boom
			case "metrics":
				source.metricErr = boom
			case "period":
				source.periodErr = boom
			case "orphans":
				source.orphanErr = boom
			}
			calls := 0
			now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.FixedZone("local", 3600))
			app := New(source, nil, func() time.Time { calls++; return now })
			snap, err := app.Load(context.Background(), LoadOptions{RootOnly: true, CostPeriod: CostPeriodWeek})
			if field == "listing" || field == "metrics" || field == "period" {
				if snap != nil || !errors.Is(err, boom) {
					t.Fatalf("fatal data returned: %+v %v", snap, err)
				}
			} else {
				if err != nil || snap.PeriodCost == nil || snap.PeriodCost.Amount != 0 {
					t.Fatalf("known zero lost: %+v %v", snap, err)
				}
				if field == "orphans" {
					if snap.Orphans != nil || snap.Capabilities.Orphans.Available || len(snap.Diagnostics) != 1 || !errors.Is(snap.Diagnostics[0], boom) {
						t.Fatalf("orphan error hidden: %+v", snap)
					}
					if _, err := app.Prune(context.Background()); !errors.Is(err, boom) || source.prune != 0 {
						t.Fatalf("failed stats allowed prune: %v", err)
					}
				}
			}
			if calls != 1 || source.start.Weekday() != time.Monday || source.start.Hour() != 0 || !source.end.Equal(now) {
				t.Fatal("incorrect period clock/bounds")
			}
		})
	}
}
func TestDeleteRefreshAndPerCallTimeout(t *testing.T) {
	boom := errors.New("delete failed")
	source := &capableSource{listSource: listSource{id: "a"}, deleteErr: boom}
	app := New(source, nil, nil)
	refs := []session.Ref{{Source: "a", ID: "one"}, {Source: "a", ID: "two"}}
	result := app.Delete(context.Background(), refs, LoadOptions{})
	if result.Count != 0 || result.Snapshot == nil || result.Snapshot.TotalCount != 0 || !errors.Is(result.Err, boom) || source.lists != 1 || !reflect.DeepEqual(source.deletes, []string{"one", "two"}) {
		t.Fatalf("partial failure: %+v", result)
	}
	for _, duration := range source.deadlines {
		if duration <= 29*time.Second || duration > deleteTimeout {
			t.Fatalf("per-delete deadline: %s", duration)
		}
	}
	source.deleteErr = nil
	source.refreshErr = boom
	result = app.Delete(context.Background(), refs[:1], LoadOptions{})
	if result.Count != 1 || result.Snapshot != nil || !errors.Is(result.Err, boom) {
		t.Fatalf("failed refresh: %+v", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := len(source.deletes)
	result = app.Delete(ctx, refs, LoadOptions{})
	if !errors.Is(result.Err, context.Canceled) || len(source.deletes) != before {
		t.Fatalf("canceled delete: %+v", result)
	}
	if err := app.Resume(ctx, refs[0]); !errors.Is(err, context.Canceled) || len(source.resumes) != 0 {
		t.Fatal("canceled resume acted")
	}
}
