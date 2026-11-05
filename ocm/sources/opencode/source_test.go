package opencode

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/kimoofey/tui/ocm/session"
)

func TestSourceIdentityAndCanceledCapabilities(t *testing.T) {
	// The adapter's constructor already cleans absolute paths; URI encoding keeps
	// embedded punctuation from becoming ambiguous source delimiters.
	path := filepath.Join(t.TempDir(), "db # ? with spaces.db")
	store := &Store{storage: storageContext{DBPath: path, DataDir: filepath.Dir(path), Executable: "agent"}}
	want := session.SourceID("opencode:" + (&url.URL{Scheme: "file", Path: path}).String())
	if store.Info().ID != want || store.Info().ID == (&Store{storage: storageContext{DBPath: path + "other"}}).Info().ID {
		t.Fatal("source path identity lost")
	}
	if _, err := store.InspectOrphans(context.Background()); !errors.Is(err, session.ErrUnsupported) {
		t.Fatalf("custom orphan stats: %v", err)
	}
	if _, err := store.Prune(context.Background()); !errors.Is(err, session.ErrUnsupported) {
		t.Fatalf("custom prune: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Resume(ctx, "same"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestAdapterCapabilitiesHonorCanceledQueries(t *testing.T) {
	// Reuse the V1 fixture to test the public capability boundary against real SQL.
	store := createTestStore(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.List(ctx, session.ListOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("list cancellation: %v", err)
	}
	if _, err := store.Metrics(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("metrics cancellation: %v", err)
	}
	if _, err := store.PeriodCost(ctx, time.Time{}, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cost cancellation: %v", err)
	}
	if _, err := store.Vacuum(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("vacuum cancellation: %v", err)
	}
}

func TestListingTiesAndMissingDiffDirectory(t *testing.T) {
	store := createTestStore(t, []sessionRow{{ID: "a", Title: "A", Directory: "/p", Updated: time.UnixMilli(20)}, {ID: "b", Title: "B", Directory: "/p", Updated: time.UnixMilli(20)}})
	listing, err := store.List(context.Background(), session.ListOptions{})
	if err != nil || listing.TotalCount != 2 || len(listing.Sessions) != 2 || listing.Sessions[0].ID != "b" || listing.Sessions[1].ID != "a" {
		t.Fatalf("tie ordering: %+v %v", listing, err)
	}
	if listing.Sessions[0].Source != store.Info().ID || listing.Sessions[0].Cost == nil || listing.Sessions[0].Cost.Amount != 0 {
		t.Fatal("source reference or known zero lost")
	}
	// Default fixture storage is eligible for maintenance, even with no diff directory.
	orphans, err := store.InspectOrphans(context.Background())
	if err != nil || orphans.Count != 0 || orphans.Bytes != 0 {
		t.Fatalf("missing diff inspection: %+v %v", orphans, err)
	}
	pruned, err := store.Prune(context.Background())
	if err != nil || pruned.Count != 0 || pruned.Bytes != 0 {
		t.Fatalf("missing diff prune: %+v %v", pruned, err)
	}
}
