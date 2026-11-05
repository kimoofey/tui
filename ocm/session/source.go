package session

import (
	"context"
	"errors"
	"time"

	"github.com/kimoofey/tui/internal/terminal"
)

var ErrUnsupported = errors.New("unsupported capability")
var ErrForeignSource = errors.New("reference belongs to another source")

type Source interface {
	Info() Info
	List(context.Context, ListOptions) (Listing, error)
	Close() error
}
type Resumer interface {
	Resume(context.Context, string) (terminal.Command, error)
}
type Deleter interface {
	Delete(context.Context, string) error
}
type PeriodCoster interface {
	PeriodCost(context.Context, time.Time, time.Time) (RecordedCost, error)
}
type StorageMetrics interface {
	Metrics(context.Context) (Metrics, error)
}
type Vacuumer interface {
	VacuumAvailability() Availability
	Vacuum(context.Context) (Metrics, error)
}
type OrphanMaintainer interface {
	OrphanAvailability() Availability
	InspectOrphans(context.Context) (Orphans, error)
	Prune(context.Context) (Orphans, error)
}
