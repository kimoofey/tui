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

const (
	reviewTimelineEventWindow = 10
	reviewFilesPageSize       = 50
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

type ghRequestedReviewer struct {
	Login string `json:"login"`
	Slug  string `json:"slug"`
}

type ghReviewRequestedEvent struct {
	CreatedAt         string               `json:"createdAt"`
	RequestedReviewer *ghRequestedReviewer `json:"requestedReviewer"`
}

type ghPRFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type ghPRNode struct {
	ID                string   `json:"id"`
	Number            int      `json:"number"`
	Title             string   `json:"title"`
	URL               string   `json:"url"`
	HeadRefOID        string   `json:"headRefOid"`
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
	TimelineItems struct {
		Nodes []ghReviewRequestedEvent `json:"nodes"`
	} `json:"timelineItems"`
	Files struct {
		Nodes []ghPRFile `json:"nodes"`
		PageInfo struct {
			HasNextPage bool `json:"hasNextPage"`
		} `json:"pageInfo"`
	} `json:"files"`
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

// queryReviewPRsLight fetches review-queue PRs with cursor pagination.
// Used by both fetchAssignedPRs (direct/team assignments) and fetchWatchPRs.
const queryReviewPRsLight = `
query($searchQuery: String!, $pageSize: Int!, $maxReviewers: Int!, $user: String!, $cursor: String) {
  search(query: $searchQuery, type: ISSUE, first: $pageSize, after: $cursor) {
    nodes {
      ... on PullRequest {
        id
        number
        title
        url
        headRefOid
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

var queryReviewPRsEnrich = fmt.Sprintf(`
query($searchQuery: String!, $pageSize: Int!, $maxReviewers: Int!, $user: String!, $cursor: String) {
  search(query: $searchQuery, type: ISSUE, first: $pageSize, after: $cursor) {
    nodes {
      ... on PullRequest {
        id
        number
        title
        url
        headRefOid
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
        timelineItems(itemTypes: [REVIEW_REQUESTED_EVENT], last: %d) {
          nodes {
            ... on ReviewRequestedEvent {
              createdAt
              requestedReviewer {
                ... on User { login }
                ... on Team { slug }
              }
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
        files(first: %d) {
          nodes {
            path
            additions
            deletions
          }
          pageInfo { hasNextPage }
        }
        viewerReview: reviews(first: 1, author: $user) {
          nodes { state }
        }
      }
    }
    pageInfo { hasNextPage endCursor }
  }
}`,
	reviewTimelineEventWindow,
	reviewFilesPageSize,
)

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
        headRefOid
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
	totalStart := time.Now()
	defer debugDuration("fetch.total", totalStart)

	userStart := time.Now()
	user, err := getUser()
	debugDuration("fetch.get_user", userStart)
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
		start := time.Now()
		prs, err := fetchAssignedPRs(cfg, user, dateThreshold)
		debugDuration("fetch.assigned.total", start)
		assignedCh <- partial{prs, err}
	}()
	go func() {
		start := time.Now()
		prs, err := fetchWatchPRs(cfg, user, dateThreshold)
		debugDuration("fetch.watch.total", start)
		watchCh <- partial{prs, err}
	}()
	go func() {
		start := time.Now()
		prs, err := fetchMyPRs(cfg)
		debugDuration("fetch.my_prs.total", start)
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
	reviewPRs = groupReviewPRs(reviewPRs)

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

func EnrichReviewPRs(cfg Config, reviewPRs []PullRequest) enrichResult {
	totalStart := time.Now()
	defer debugDuration("enrich.total", totalStart)

	userStart := time.Now()
	user, err := getUser()
	debugDuration("enrich.get_user", userStart)
	if err != nil {
		return enrichResult{Err: fmt.Errorf("getting GitHub user: %w", err)}
	}

	dateThreshold := time.Now().AddDate(0, 0, -cfg.DaysAgo).Format("2006-01-02")

	type partial struct {
		prs []PullRequest
		err error
	}
	assignedCh := make(chan partial, 1)
	watchCh := make(chan partial, 1)

	go func() {
		start := time.Now()
		prs, err := fetchAssignedPRsEnriched(cfg, user, dateThreshold)
		debugDuration("enrich.assigned.total", start)
		assignedCh <- partial{prs, err}
	}()
	go func() {
		start := time.Now()
		prs, err := fetchWatchPRsEnriched(cfg, user, dateThreshold)
		debugDuration("enrich.watch.total", start)
		watchCh <- partial{prs, err}
	}()

	assigned := <-assignedCh
	watch := <-watchCh

	updates := make(map[string]PullRequest)
	allowed := make(map[string]struct{}, len(reviewPRs))
	for _, pr := range reviewPRs {
		if pr.URL == "" {
			continue
		}
		allowed[pr.URL] = struct{}{}
	}

	for _, pr := range assigned.prs {
		if _, ok := allowed[pr.URL]; ok {
			updates[pr.URL] = pr
		}
	}
	for _, pr := range watch.prs {
		if _, ok := allowed[pr.URL]; ok {
			if _, exists := updates[pr.URL]; !exists {
				updates[pr.URL] = pr
			}
		}
	}

	var errs []string
	if assigned.err != nil {
		errs = append(errs, assigned.err.Error())
	}
	if watch.err != nil {
		errs = append(errs, watch.err.Error())
	}
	if len(errs) > 0 {
		return enrichResult{Updates: updates, Err: errors.New(strings.Join(errs, "; "))}
	}
	if DebugEnabled {
		log.Printf("[enrich] updates=%d requested=%d", len(updates), len(reviewPRs))
	}

	return enrichResult{Updates: updates}
}

// fetchReviewNodes runs queryReviewPRs with cursor pagination and returns the
// raw nodes. Used by both fetchAssignedPRs and fetchWatchPRs.
func fetchReviewNodes(cfg Config, user, stage, searchQuery, query string) ([]ghPRNode, error) {
	var allNodes []ghPRNode
	cursor := ""
	pagesFetched := 0

	for page := 0; page < cfg.MaxPages; page++ {
		pagesFetched++
		pageStart := time.Now()
		args := []string{
			"-f", "query=" + query,
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
			debugDuration(fmt.Sprintf("%s.page_%d", stage, page+1), pageStart)
			return nil, err
		}

		var resp pagedSearchResponse
		if err := json.Unmarshal(out, &resp); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		if err := resp.graphQLErr(); err != nil {
			debugDuration(fmt.Sprintf("%s.page_%d", stage, page+1), pageStart)
			return nil, err
		}
		debugDuration(fmt.Sprintf("%s.page_%d", stage, page+1), pageStart)

		allNodes = append(allNodes, resp.Data.Search.Nodes...)

		if !resp.Data.Search.PageInfo.HasNextPage {
			break
		}
		cursor = resp.Data.Search.PageInfo.EndCursor
	}
	if DebugEnabled {
		log.Printf("[timing] %s.pages=%d nodes=%d", stage, pagesFetched, len(allNodes))
	}

	return allNodes, nil
}

func fetchAssignedPRs(cfg Config, user, dateThreshold string) ([]PullRequest, error) {
	searchQuery := assignedSearchQuery(cfg, user, dateThreshold)

	nodes, err := fetchReviewNodes(cfg, user, "fetch.assigned", searchQuery, queryReviewPRsLight)
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
		pr, err := convertPR(node, bucketForNode(node, user), approvals, user)
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
	searchQuery := watchSearchQuery(cfg, user, dateThreshold, repoParts)

	nodes, err := fetchReviewNodes(cfg, user, "fetch.watch", searchQuery, queryReviewPRsLight)
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
		pr, err := convertPR(node, BucketWatch, approvals, user)
		if err != nil {
			continue
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

func fetchAssignedPRsEnriched(cfg Config, user, dateThreshold string) ([]PullRequest, error) {
	searchQuery := assignedSearchQuery(cfg, user, dateThreshold)

	nodes, err := fetchReviewNodes(cfg, user, "enrich.assigned", searchQuery, queryReviewPRsEnrich)
	if err != nil {
		return nil, err
	}

	prs := make([]PullRequest, 0, len(nodes))
	for _, node := range nodes {
		if node.Number == 0 {
			continue
		}
		approvals, userReviewed := reviewCounts(node)
		if !shouldInclude(node, cfg, approvals, userReviewed) {
			continue
		}
		pr, err := convertPR(node, bucketForNode(node, user), approvals, user)
		if err != nil {
			continue
		}
		pr.Enriched = true
		prs = append(prs, pr)
	}
	return prs, nil
}

func fetchWatchPRsEnriched(cfg Config, user, dateThreshold string) ([]PullRequest, error) {
	if len(cfg.WatchRepos) == 0 {
		return nil, nil
	}

	var repoParts []string
	for _, r := range cfg.WatchRepos {
		repoParts = append(repoParts, "repo:"+r)
	}
	searchQuery := watchSearchQuery(cfg, user, dateThreshold, repoParts)

	nodes, err := fetchReviewNodes(cfg, user, "enrich.watch", searchQuery, queryReviewPRsEnrich)
	if err != nil {
		return nil, err
	}

	prs := make([]PullRequest, 0, len(nodes))
	for _, node := range nodes {
		if node.Number == 0 {
			continue
		}
		approvals, userReviewed := reviewCounts(node)
		if !shouldInclude(node, cfg, approvals, userReviewed) {
			continue
		}
		pr, err := convertPR(node, BucketWatch, approvals, user)
		if err != nil {
			continue
		}
		pr.Enriched = true
		prs = append(prs, pr)
	}
	return prs, nil
}

func fetchMyPRs(cfg Config) ([]PullRequest, error) {
	searchQuery := "is:pr is:open author:@me sort:created-asc"

	var allNodes []ghPRNode
	cursor := ""

	for page := 0; page < cfg.MaxPages; page++ {
		pageStart := time.Now()
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
			debugDuration(fmt.Sprintf("fetch.my_prs.page_%d", page+1), pageStart)
			return nil, err
		}

		var resp pagedSearchResponse
		if err := json.Unmarshal(out, &resp); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		if err := resp.graphQLErr(); err != nil {
			debugDuration(fmt.Sprintf("fetch.my_prs.page_%d", page+1), pageStart)
			return nil, err
		}
		debugDuration(fmt.Sprintf("fetch.my_prs.page_%d", page+1), pageStart)

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

func debugDuration(stage string, start time.Time) {
	if !DebugEnabled {
		return
	}
	log.Printf("[timing] %s=%s", stage, time.Since(start).Round(time.Millisecond))
}

func assignedSearchQuery(cfg Config, user, dateThreshold string) string {
	return fmt.Sprintf(
		"is:pr is:open review-requested:@me -author:%s -draft:true created:>=%s%s sort:created-asc",
		user, dateThreshold, searchFilterSuffix(cfg),
	)
}

func watchSearchQuery(cfg Config, user, dateThreshold string, repoParts []string) string {
	return fmt.Sprintf(
		"%s is:pr is:open created:>=%s -author:%s -draft:true%s sort:created-asc",
		strings.Join(repoParts, " "), dateThreshold, user, searchFilterSuffix(cfg),
	)
}

func searchFilterSuffix(cfg Config) string {
	parts := []string{}
	if cfg.MinApprovals > 0 {
		parts = append(parts, "-review:approved")
	}
	if cfg.SkipBots {
		parts = append(parts,
			"-author:app/dependabot",
			"-author:app/renovate",
			"-author:app/github-actions",
		)
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
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

func PrefetchUser() error {
	_, err := getUser()
	return err
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

func convertPR(node ghPRNode, bucket Bucket, approvals int, user string) (PullRequest, error) {
	t, err := time.Parse(time.RFC3339, node.CreatedAt)
	if err != nil {
		return PullRequest{}, fmt.Errorf("parsing createdAt %q: %w", node.CreatedAt, err)
	}
	waitSince := selectWaitSince(node, bucket, user, t)
	files := make([]PRFile, 0, len(node.Files.Nodes))
	for _, f := range node.Files.Nodes {
		files = append(files, PRFile{Path: f.Path, Additions: f.Additions, Deletions: f.Deletions})
	}
	return PullRequest{
		NodeID:       node.ID,
		Number:       node.Number,
		Title:        node.Title,
		URL:          node.URL,
		Author:       node.Author.Login,
		Repo:         node.Repository.NameWithOwner,
		HeadRefOID:   node.HeadRefOID,
		CreatedAt:    t,
		WaitSince:    waitSince,
		Approvals:    approvals,
		Additions:    node.Additions,
		Deletions:    node.Deletions,
		ChangedFiles: node.ChangedFiles,
		Files:        files,
		FilesTruncated: node.Files.PageInfo.HasNextPage,
		Bucket:       bucket,
		Enriched:     len(node.Files.Nodes) > 0 || node.Files.PageInfo.HasNextPage || len(node.TimelineItems.Nodes) > 0,
	}, nil
}

func selectWaitSince(node ghPRNode, bucket Bucket, user string, fallback time.Time) time.Time {
	latest := time.Time{}
	for _, item := range node.TimelineItems.Nodes {
		if bucket != BucketWatch {
			if item.RequestedReviewer == nil {
				continue
			}
			if !matchesWaitRequest(item.RequestedReviewer, bucket, user, node) {
				continue
			}
		}
		t, err := time.Parse(time.RFC3339, item.CreatedAt)
		if err != nil {
			continue
		}
		if t.After(latest) {
			latest = t
		}
	}

	if latest.IsZero() {
		return fallback
	}
	return latest
}

func matchesWaitRequest(requested *ghRequestedReviewer, bucket Bucket, user string, node ghPRNode) bool {
	if bucket == BucketDirect {
		return requested.Login == user
	}
	if bucket != BucketTeam {
		return false
	}
	teams := currentRequestedTeams(node)
	if len(teams) == 0 {
		return false
	}
	_, ok := teams[requested.Slug]
	return ok
}

func currentRequestedTeams(node ghPRNode) map[string]struct{} {
	teams := make(map[string]struct{})
	for _, rr := range node.ReviewRequests.Nodes {
		slug := rr.RequestedReviewer.Slug
		if slug != "" {
			teams[slug] = struct{}{}
		}
	}
	return teams
}

func convertMyPR(node ghPRNode) (PullRequest, error) {
	t, err := time.Parse(time.RFC3339, node.CreatedAt)
	if err != nil {
		return PullRequest{}, fmt.Errorf("parsing createdAt %q: %w", node.CreatedAt, err)
	}
	return PullRequest{
		NodeID:         node.ID,
		Number:         node.Number,
		Title:          node.Title,
		URL:            node.URL,
		Author:         node.Author.Login,
		Repo:           node.Repository.NameWithOwner,
		HeadRefOID:     node.HeadRefOID,
		CreatedAt:      t,
		Bucket:         BucketMine,
		IsDraft:        node.IsDraft,
		ReviewDecision: node.ReviewDecision,
	}, nil
}
