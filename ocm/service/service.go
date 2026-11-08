package service

import (
	"errors"
	"fmt"
	"github.com/kimoofey/tui/ocm/history"
	"time"

	"github.com/kimoofey/tui/internal/terminal"
	"github.com/kimoofey/tui/ocm/session"
)

type Launcher interface{ Launch(terminal.Command) error }
type Service struct {
	history  *historyWorker
	source   session.Source
	launcher Launcher
	now      func() time.Time
}

func New(source session.Source, launcher Launcher, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{source: source, launcher: launcher, now: now}
}
func (s *Service) validate(ref session.Ref) error {
	if ref.Source != s.source.Info().ID {
		return fmt.Errorf("%w: %s", session.ErrForeignSource, ref.Source)
	}
	return nil
}

type Capabilities struct {
	Resume     bool
	Delete     bool
	RootFilter bool
	PeriodCost bool
	Metrics    bool
	Vacuum     session.Availability
	Orphans    session.Availability
}

func (s *Service) Capabilities() Capabilities {
	c := Capabilities{RootFilter: s.source.Info().RootFilter}
	_, c.Resume = s.source.(session.Resumer)
	_, c.Delete = s.source.(session.Deleter)
	if s.history != nil {
		s.history.mu.Lock()
		c.Delete = c.Delete && s.history.guard.Available && !errors.Is(s.history.status.Err, history.ErrBinding)
		s.history.mu.Unlock()
	}
	_, c.PeriodCost = s.source.(session.PeriodCoster)
	_, c.Metrics = s.source.(session.StorageMetrics)
	if v, ok := s.source.(session.Vacuumer); ok && c.Metrics {
		c.Vacuum = v.VacuumAvailability()
	}
	if o, ok := s.source.(session.OrphanMaintainer); ok && c.Metrics {
		c.Orphans = o.OrphanAvailability()
	}
	return c
}
func unavailable(action string, a session.Availability) error {
	return fmt.Errorf("%w: %s unavailable: %s", session.ErrUnsupported, action, a.Reason)
}
