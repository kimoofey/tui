//go:build !mock

package prq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type ghAuthor struct {
	TypeName string `json:"__typename"`
	Login    string `json:"login"`
}

type ghReview struct {
	State  string   `json:"state"`
	Author ghAuthor `json:"author"`
}

type ghReviewRequest struct {
	RequestedReviewer struct {
		Login string `json:"login"` // User
		Slug  string `json:"slug"`  // Team
	} `json:"requestedReviewer"`
}

type ghPRNode struct {
	Number            int      `json:"number"`
	Title             string   `json:"title"`
	URL               string   `json:"url"`
	Author            ghAuthor `json:"author"`
	CreatedAt         string   `json:"createdAt"`
	Additions         int      `json:"additions"`
	Deletions         int      `json:"deletions"`
	ChangedFiles      int      `json:"changedFiles"`
	IsDraft           bool     `json:"isDraft"`
	ReviewDecision    string   `json:"reviewDecision"`
	StatusCheckRollup *struct {
		State string `json:"state"`
	} `json:"statusCheckRollup"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	ReviewRequests struct {
		Nodes []ghReviewRequest `json:"nodes"`
	} `json:"reviewRequests"`
	Reviews struct {
		Nodes []ghReview `json:"nodes"`
	} `json:"latestOpinionatedReviews"`
	ViewerReview struct {
		Nodes []ghReview `json:"nodes"`
	} `json:"viewerReview"`
}

// graphQLErrors captures GitHub's top-level "errors" array (HTTP 200 with errors).
type graphQLErrors struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// graphQLErr returns an error if the response contains GraphQL errors.
func (g graphQLErrors) graphQLErr() error {
	if len(g.Errors) == 0 {
		return nil
	}
	msgs := make([]string, len(g.Errors))
	for i, e := range g.Errors {
		msgs[i] = e.Message
	}
	return fmt.Errorf("github: %s", strings.Join(msgs, "; "))
}

type pagedSearchResponse struct {
	graphQLErrors
	Data struct {
		Search struct {
			Nodes    []ghPRNode `json:"nodes"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"search"`
	} `json:"data"`
}

// queryReviewPRs fetches review-queue PRs with cursor pagination.
// Used by both fetchAssignedPRs (direct/team assignments) and fetchWatchPRs.
const queryReviewPRs = `
query($searchQuery: String!, $pageSize: Int!, $maxReviewers: Int!, $user: String!, $cursor: String) {
  search(query: $searchQuery, type: ISSUE, first: $pageSize, after: $cursor) {
    nodes {
      ... on PullRequest {
        number
        title
        url
        author { __typename login }
        createdAt
        repository { nameWithOwner }
        reviewRequests(first: $maxReviewers) {
          nodes {
            requestedReviewer {
              ... on User { login }
              ... on Team { slug }
            }
          }
        }
        latestOpinionatedReviews(first: $maxReviewers) {
          nodes {
            state
            author { login }
          }
        }
        additions
        deletions
        changedFiles
        viewerReview: reviews(first: 1, author: $user) {
          nodes { state }
        }
      }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// queryMyPRs fetches open PRs authored by the current user, with review decision
// and draft status. No date filter — users may have long-lived own PRs.
const queryMyPRs = `
query($searchQuery: String!, $pageSize: Int!, $cursor: String) {
  search(query: $searchQuery, type: ISSUE, first: $pageSize, after: $cursor) {
    nodes {
      ... on PullRequest {
        number
        title
        url
        isDraft
        createdAt
        reviewDecision
        repository { nameWithOwner }
        author { login }
        statusCheckRollup { state }
      }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// FetchAll fetches S1 (direct/team review requests), Watch (combined watch repos),
// and S3 (user's own PRs) in 3 parallel goroutines, then merges with dedup.
// S1 results take priority over Watch in case of overlap.
func FetchAll(cfg Config) FetchResult {
	user, err := getUser()
	if err != nil {
		return FetchResult{Err: fmt.Errorf("getting GitHub user: %w", err)}
	}

	dateThreshold := time.Now().AddDate(0, 0, -cfg.DaysAgo).Format("2006-01-02")

	type partial struct {
		prs []PullRequest
		err error
	}
	assignedCh := make(chan partial, 1)
	watchCh := make(chan partial, 1)
	myCh := make(chan partial, 1)

	go func() {
		prs, err := fetchAssignedPRs(cfg, user, dateThreshold)
		assignedCh <- partial{prs, err}
	}()
	go func() {
		prs, err := fetchWatchPRs(cfg, user, dateThreshold)
		watchCh <- partial{prs, err}
	}()
	go func() {
		prs, err := fetchMyPRs(cfg)
		myCh <- partial{prs, err}
	}()

	assigned := <-assignedCh
	watch := <-watchCh
	my := <-myCh

	// Merge assigned + watch with dedup. Assigned wins on overlap (iterated first).
	seen := make(map[string]bool, len(assigned.prs)+len(watch.prs))
	var reviewPRs []PullRequest
	for _, pr := range assigned.prs {
		if !seen[pr.URL] {
			seen[pr.URL] = true
			reviewPRs = append(reviewPRs, pr)
		}
	}
	for _, pr := range watch.prs {
		if !seen[pr.URL] {
			seen[pr.URL] = true
			reviewPRs = append(reviewPRs, pr)
		}
	}

	if DebugEnabled {
		log.Printf("[fetch] assigned=%d watch=%d myPRs=%d reviewPRs(merged)=%d",
			len(assigned.prs), len(watch.prs), len(my.prs), len(reviewPRs))
		for _, pr := range reviewPRs {
			approvalStr := fmt.Sprintf("%d/%d", pr.Approvals, cfg.MinApprovals)
			if pr.Approvals >= cfg.MinApprovals {
				approvalStr += " ✓"
			}
			log.Printf("[pr] #%d %s %s %q @%s %s %s",
				pr.Number, bucketLabel(pr.Bucket), pr.RepoShort(),
				pr.Title, pr.Author, pr.Age(), approvalStr)
		}
	}

	// Collect first error (non-fatal — partial results are still returned).
	var errs []string
	if assigned.err != nil {
		errs = append(errs, assigned.err.Error())
	}
	if watch.err != nil {
		errs = append(errs, watch.err.Error())
	}
	if my.err != nil {
		errs = append(errs, my.err.Error())
	}
	var fetchErr error
	if len(errs) > 0 {
		fetchErr = errors.New(strings.Join(errs, "; "))
	}

	return FetchResult{ReviewPRs: reviewPRs, MyPRs: my.prs, Err: fetchErr}
}

// fetchReviewNodes runs queryReviewPRs with cursor pagination and returns the
// raw nodes. Used by both fetchAssignedPRs and fetchWatchPRs.
func fetchReviewNodes(cfg Config, user, searchQuery string) ([]ghPRNode, error) {
	var allNodes []ghPRNode
	cursor := ""

	for page := 0; page < cfg.MaxPages; page++ {
		args := []string{
			"-f", "query=" + queryReviewPRs,
			"-f", "searchQuery=" + searchQuery,
			"-f", "user=" + user,
			"-F", fmt.Sprintf("pageSize=%d", cfg.PageSize),
			"-F", fmt.Sprintf("maxReviewers=%d", cfg.MaxReviewers),
		}
		if cursor != "" {
			args = append(args, "-f", "cursor="+cursor)
		}

		out, err := runGraphQL(args)
		if err != nil {
			return nil, err
		}

		var resp pagedSearchResponse
		if err := json.Unmarshal(out, &resp); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		if err := resp.graphQLErr(); err != nil {
			return nil, err
		}

		allNodes = append(allNodes, resp.Data.Search.Nodes...)

		if !resp.Data.Search.PageInfo.HasNextPage {
			break
		}
		cursor = resp.Data.Search.PageInfo.EndCursor
	}

	return allNodes, nil
}

func fetchAssignedPRs(cfg Config, user, dateThreshold string) ([]PullRequest, error) {
	searchQuery := fmt.Sprintf(
		"is:pr is:open review-requested:@me -author:%s -draft:true created:>=%s sort:created-asc",
		user, dateThreshold,
	)

	nodes, err := fetchReviewNodes(cfg, user, searchQuery)
	if err != nil {
		return nil, err
	}

	var prs []PullRequest
	for _, node := range nodes {
		if node.Number == 0 {
			continue // non-PR search hit
		}
		approvals, userReviewed := reviewCounts(node)
		if !shouldInclude(node, cfg, approvals, userReviewed) {
			continue
		}
		pr, err := convertPR(node, bucketForNode(node, user), approvals)
		if err != nil {
			continue
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

func fetchWatchPRs(cfg Config, user, dateThreshold string) ([]PullRequest, error) {
	if len(cfg.WatchRepos) == 0 {
		return nil, nil
	}

	var repoParts []string
	for _, r := range cfg.WatchRepos {
		repoParts = append(repoParts, "repo:"+r)
	}
	searchQuery := fmt.Sprintf(
		"%s is:pr is:open created:>=%s -author:%s -draft:true sort:created-asc",
		strings.Join(repoParts, " "), dateThreshold, user,
	)

	nodes, err := fetchReviewNodes(cfg, user, searchQuery)
	if err != nil {
		return nil, err
	}

	var prs []PullRequest
	for _, node := range nodes {
		if node.Number == 0 {
			continue
		}
		approvals, userReviewed := reviewCounts(node)
		if !shouldInclude(node, cfg, approvals, userReviewed) {
			continue
		}
		pr, err := convertPR(node, BucketWatch, approvals)
		if err != nil {
			continue
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

func fetchMyPRs(cfg Config) ([]PullRequest, error) {
	searchQuery := "is:pr is:open author:@me sort:created-asc"

	var allNodes []ghPRNode
	cursor := ""

	for page := 0; page < cfg.MaxPages; page++ {
		args := []string{
			"-f", "query=" + queryMyPRs,
			"-f", "searchQuery=" + searchQuery,
			"-F", fmt.Sprintf("pageSize=%d", cfg.PageSize),
		}
		if cursor != "" {
			args = append(args, "-f", "cursor="+cursor)
		}

		out, err := runGraphQL(args)
		if err != nil {
			return nil, err
		}

		var resp pagedSearchResponse
		if err := json.Unmarshal(out, &resp); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		if err := resp.graphQLErr(); err != nil {
			return nil, err
		}

		allNodes = append(allNodes, resp.Data.Search.Nodes...)

		if !resp.Data.Search.PageInfo.HasNextPage {
			break
		}
		cursor = resp.Data.Search.PageInfo.EndCursor
	}

	var prs []PullRequest
	for _, node := range allNodes {
		if node.Number == 0 {
			continue
		}
		pr, err := convertMyPR(node)
		if err != nil {
			continue
		}
		prs = append(prs, pr)
	}

	if DebugEnabled {
		for _, pr := range prs {
			log.Printf("[my-pr] #%d %s %q draft=%v review=%s",
				pr.Number, pr.RepoShort(), pr.Title, pr.IsDraft, pr.ReviewDecision)
		}
	}

	return prs, nil
}

var (
	cachedUser   string
	cachedUserMu sync.Mutex
)

const cmdTimeout = 30 * time.Second

func getUser() (string, error) {
	cachedUserMu.Lock()
	defer cachedUserMu.Unlock()
	if cachedUser != "" {
		return cachedUser, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gh", "api", "user", "-q", ".login").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("gh api user: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("gh api user: %w", err)
	}
	cachedUser = strings.TrimSpace(string(out))
	return cachedUser, nil
}

func runGraphQL(args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", append([]string{"api", "graphql"}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, errors.New(strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

func shouldInclude(node ghPRNode, cfg Config, approvals int, userReviewed bool) bool {
	if cfg.SkipBots && node.Author.TypeName == "Bot" {
		return false
	}
	if cfg.SkipAlreadyReviewed && userReviewed {
		return false
	}
	if approvals >= cfg.MinApprovals {
		return false
	}
	return true
}

func reviewCounts(node ghPRNode) (approvals int, userReviewed bool) {
	for _, r := range node.Reviews.Nodes {
		if r.State == "APPROVED" {
			approvals++
		}
	}
	userReviewed = len(node.ViewerReview.Nodes) > 0
	return
}

func bucketForNode(node ghPRNode, user string) Bucket {
	for _, rr := range node.ReviewRequests.Nodes {
		if rr.RequestedReviewer.Login == user {
			return BucketDirect
		}
	}
	return BucketTeam
}

func convertPR(node ghPRNode, bucket Bucket, approvals int) (PullRequest, error) {
	t, err := time.Parse(time.RFC3339, node.CreatedAt)
	if err != nil {
		return PullRequest{}, fmt.Errorf("parsing createdAt %q: %w", node.CreatedAt, err)
	}
	return PullRequest{
		Number:       node.Number,
		Title:        node.Title,
		URL:          node.URL,
		Author:       node.Author.Login,
		Repo:         node.Repository.NameWithOwner,
		CreatedAt:    t,
		Approvals:    approvals,
		Additions:    node.Additions,
		Deletions:    node.Deletions,
		ChangedFiles: node.ChangedFiles,
		Bucket:       bucket,
	}, nil
}

func convertMyPR(node ghPRNode) (PullRequest, error) {
	t, err := time.Parse(time.RFC3339, node.CreatedAt)
	if err != nil {
		return PullRequest{}, fmt.Errorf("parsing createdAt %q: %w", node.CreatedAt, err)
	}
	return PullRequest{
		Number:         node.Number,
		Title:          node.Title,
		URL:            node.URL,
		Author:         node.Author.Login,
		Repo:           node.Repository.NameWithOwner,
		CreatedAt:      t,
		Bucket:         BucketMine,
		IsDraft:        node.IsDraft,
		ReviewDecision: node.ReviewDecision,
	}, nil
}
