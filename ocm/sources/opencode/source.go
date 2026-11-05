package opencode

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/kimoofey/tui/internal/terminal"
	"github.com/kimoofey/tui/ocm/session"
)

func Discover() (*Store, error) {
	storage, err := discover()
	if err != nil {
		return nil, fmt.Errorf("resolving db path: %w", err)
	}
	return openStorage(storage)
}

func (s *Store) Info() session.Info {
	return session.Info{ID: session.SourceID("opencode:" + (&url.URL{Scheme: "file", Path: s.storage.DBPath}).String()), Kind: "opencode", Name: "OpenCode", RootFilter: true}
}

func (s *Store) List(ctx context.Context, opts session.ListOptions) (session.Listing, error) {
	rows, err := s.loadSessions(ctx, opts.RootOnly)
	if err != nil {
		return session.Listing{}, err
	}
	count, err := s.sessionCount(ctx)
	if err != nil {
		return session.Listing{}, err
	}
	sessions := make([]session.Session, len(rows))
	for i, row := range rows {
		sessions[i] = session.Session{Ref: session.Ref{Source: s.Info().ID, ID: row.ID}, Title: row.Title, Directory: row.Directory, Created: row.Created, Updated: row.Updated, Cost: &session.RecordedCost{Amount: row.Cost, Currency: "USD", Provenance: "source-recorded"}}
	}
	return session.Listing{Sessions: sessions, TotalCount: count}, nil
}
func (s *Store) Resume(ctx context.Context, id string) (terminal.Command, error) {
	if err := ctx.Err(); err != nil {
		return terminal.Command{}, err
	}
	spec := terminal.Command{Executable: s.storage.Executable, Args: sessionArgs(s.storage, "-s", id), Env: map[string]string{"OPENCODE_DB": s.storage.DBPath, "XDG_DATA_HOME": filepath.Dir(s.storage.DataDir)}}
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR"} {
		if value, ok := os.LookupEnv(key); ok {
			spec.Env[key] = value
		} else {
			spec.Unset = append(spec.Unset, key)
		}
	}
	return spec, spec.Validate()
}
func (s *Store) Delete(ctx context.Context, id string) error { return runDelete(ctx, s.storage, id) }
func (s *Store) Metrics(ctx context.Context) (session.Metrics, error) {
	used, total, err := s.dbStats(ctx)
	return session.Metrics{Used: used, Total: total}, err
}
func (s *Store) PeriodCost(ctx context.Context, start, end time.Time) (session.RecordedCost, error) {
	amount, err := s.periodCost(ctx, start.UnixMilli(), end.UnixMilli())
	return session.RecordedCost{Amount: amount, Currency: "USD", Provenance: "source-recorded"}, err
}
func (s *Store) VacuumAvailability() session.Availability {
	return session.Availability{Available: true}
}
func (s *Store) Vacuum(ctx context.Context) (session.Metrics, error) {
	used, total, err := s.vacuumDB(ctx)
	return session.Metrics{Used: used, Total: total}, err
}
func (s *Store) OrphanAvailability() session.Availability {
	if !s.storage.PruneSupported() {
		return session.Availability{Reason: "pruning unavailable for custom/channel databases: legacy diff files are shared"}
	}
	return session.Availability{Available: true}
}
func (s *Store) InspectOrphans(ctx context.Context) (session.Orphans, error) {
	if a := s.OrphanAvailability(); !a.Available {
		return session.Orphans{}, fmt.Errorf("%w: %s", session.ErrUnsupported, a.Reason)
	}
	count, bytes, err := s.orphanStats(ctx)
	return session.Orphans{Count: count, Bytes: bytes}, err
}
func (s *Store) Prune(ctx context.Context) (session.Orphans, error) {
	if a := s.OrphanAvailability(); !a.Available {
		return session.Orphans{}, fmt.Errorf("%w: %s", session.ErrUnsupported, a.Reason)
	}
	count, bytes, err := s.pruneOrphans(ctx)
	return session.Orphans{Count: count, Bytes: bytes}, err
}
