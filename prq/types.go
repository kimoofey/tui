package prq

import (
	"fmt"
	"strings"
	"time"
)

var DebugEnabled bool

type Bucket string

const (
	BucketDirect Bucket = "direct"
	BucketTeam   Bucket = "team"
	BucketWatch  Bucket = "watch"
	BucketMine   Bucket = "mine"
)

type PullRequest struct {
	NodeID         string
	Number         int
	Title          string
	URL            string
	Author         string
	Repo           string
	HeadRefOID     string
	CreatedAt      time.Time
	WaitSince      time.Time
	Approvals      int
	ReviewDecision string
	Additions      int
	Deletions      int
	ChangedFiles   int
	Files          []PRFile
	FilesTruncated bool
	Bucket         Bucket
	IsDraft        bool
	Enriched       bool
}

type PRFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

func (pr PullRequest) Age() string {
	return pr.age(time.Now())
}

func (pr PullRequest) age(now time.Time) string {
	days := int(now.Sub(pr.CreatedAt).Hours() / 24)
	switch days {
	case 0:
		return "today"
	case 1:
		return "1d ago"
	default:
		return fmt.Sprintf("%dd ago", days)
	}
}

func (pr PullRequest) WaitTime() string {
	return pr.waitTime(time.Now())
}

func (pr PullRequest) waitTime(now time.Time) string {
	start := pr.WaitSince
	if start.IsZero() {
		start = pr.CreatedAt
	}
	d := now.Sub(start)
	if d < 0 {
		d = 0
	}
	if d < time.Hour {
		return "<1h"
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}
	days := hours / 24
	if days <= 7 {
		return fmt.Sprintf("%dd", days)
	}
	return "7d+"
}

func (pr PullRequest) RepoShort() string {
	if i := strings.LastIndex(pr.Repo, "/"); i >= 0 {
		return pr.Repo[i+1:]
	}
	return pr.Repo
}

type FetchResult struct {
	ReviewPRs []PullRequest
	MyPRs     []PullRequest
	Err       error
}
