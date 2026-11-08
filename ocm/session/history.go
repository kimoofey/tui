package session

import "context"

// HistoryBinding identifies an accounting source independently of runtime Ref routing.
type HistoryBinding struct{ Path, Schema string }

// Usage contains accounting metadata only. Nil means unavailable, including invalid
// source values; a non-nil zero is a recorded zero.
type Usage struct {
	Kind, ID                                        string
	Timestamp                                       *int64
	Cost                                            *float64
	Provider, Model                                 *string
	Input, Output, Reasoning, CacheRead, CacheWrite *int64
}
type Observation struct {
	Binding       HistoryBinding
	SessionID     string
	Present       bool
	ApproximateV1 bool
	Usage         []Usage
}

// HistorySource returns complete observations or an error, never partial captures.
type HistorySource interface {
	HistorySessions(context.Context) ([]string, error)
	CaptureHistory(context.Context, string) (Observation, error)
}

// HistoryDeleter describes the known native recursive deletion scope independently
// of the filtered browser. Unsupported scope must never permit an unguarded action.
type HistoryDeleter interface {
	HistoryDeletionAvailability(context.Context) Availability
	HistoryDeletionScope(context.Context, Ref) ([]Ref, error)
}
