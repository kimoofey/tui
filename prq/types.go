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
	Number         int
	Title          string
	URL            string
	Author         string
	Repo           string
	CreatedAt      time.Time
	Approvals      int
	Bucket         Bucket
	IsDraft        bool
	ReviewDecision string
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
