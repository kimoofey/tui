package session

import "time"

type SourceID string
type Ref struct {
	Source SourceID
	ID     string
}

type RecordedCost struct {
	Amount     float64
	Currency   string
	Provenance string
}

type Session struct {
	Ref
	Title     string
	Directory string
	Created   time.Time
	Updated   time.Time
	Cost      *RecordedCost
}

type Info struct {
	ID         SourceID
	Kind       string
	Name       string
	RootFilter bool
}
type ListOptions struct{ RootOnly bool }
type Listing struct {
	Sessions   []Session
	TotalCount int
}
type Metrics struct{ Used, Total int64 }
type Orphans struct {
	Count int
	Bytes int64
}
type Availability struct {
	Available bool
	Reason    string
}
