package prq

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"github.com/kimoofey/tui/internal/ui"
)

const (
	widthSource   = 8
	minRepoWidth  = 16
	maxRepoWidth  = 24
	widthAuthor   = 27
	widthPending  = 7
	widthReview   = 9
	widthEstimate = 8

	minTitleWidth = 12
	// Give titles a readable budget before expanding the repository column.
	preferredTitleWidth = 32

	numCols = 7

	tableFixedWidth = widthSource +
		widthAuthor + widthPending + widthReview + widthEstimate + numCols*2

	borderWidth = 2
)

func repoWidth(termWidth, fixedWidth int) int {
	return min(max(termWidth-fixedWidth-2-preferredTitleWidth, minRepoWidth), maxRepoWidth)
}

func titleWidth(termWidth int) int {
	w := max(termWidth-tableFixedWidth-repoWidth(termWidth, tableFixedWidth)-2, minTitleWidth)
	return w
}

func makeColumns(termWidth int) []table.Column {
	return []table.Column{
		{Title: "Source", Width: widthSource},
		{Title: "Repo", Width: repoWidth(termWidth, tableFixedWidth)},
		{Title: "Title", Width: titleWidth(termWidth)},
		{Title: "Author", Width: widthAuthor},
		{Title: "Pending", Width: widthPending},
		{Title: "Time", Width: widthEstimate},
		{Title: "Status", Width: widthReview},
	}
}

func bucketLabel(b Bucket) string {
	switch b {
	case BucketDirect:
		return "[direct]"
	case BucketTeam:
		return "[team]  "
	case BucketWatch:
		return "[watch] "
	default:
		return fmt.Sprintf("%-8s", "["+string(b)+"]")
	}
}

// PRsToRows converts a slice of PullRequests into bubbles/table rows.
func PRsToRows(prs []PullRequest, estimateBuckets []int) []table.Row {
	rows := make([]table.Row, len(prs))
	for i, pr := range prs {
		rows[i] = table.Row{
			bucketLabel(pr.Bucket),
			pr.RepoShort(),
			pr.Title,
			"@" + ui.Truncate(pr.Author, widthAuthor-1),
			waitCell(pr),
			estimateCell(pr, estimateBuckets),
			reviewQueueStatusLabel(pr),
		}
	}
	return rows
}

func waitCell(pr PullRequest) string {
	if !pr.Enriched {
		return "..."
	}
	return pr.WaitTime()
}

func estimateCell(pr PullRequest, estimateBuckets []int) string {
	if !pr.Enriched {
		return "..."
	}
	return EstimateReviewTime(pr.Author, pr.Title, pr.Additions, pr.Deletions, pr.ChangedFiles, pr.Files, pr.FilesTruncated, estimateBuckets)
}

const (
	widthMyReview = 8

	numMyCols = 5

	myTableFixedWidth = widthSource + widthPending + widthMyReview + numMyCols*2
)

func myTitleWidth(termWidth int) int {
	w := max(termWidth-myTableFixedWidth-repoWidth(termWidth, myTableFixedWidth)-2, minTitleWidth)
	return w
}

func makeMyColumns(termWidth int) []table.Column {
	return []table.Column{
		{Title: "Status", Width: widthSource},
		{Title: "Repo", Width: repoWidth(termWidth, myTableFixedWidth)},
		{Title: "Title", Width: myTitleWidth(termWidth)},
		{Title: "Age", Width: widthPending},
		{Title: "Review", Width: widthMyReview},
	}
}

func draftLabel(isDraft bool) string {
	if isDraft {
		return "[draft] "
	}
	return "[open]  "
}

func reviewDecisionLabel(decision string) string {
	switch decision {
	case "APPROVED":
		return "approved"
	case "CHANGES_REQUESTED":
		return "changes"
	case "REVIEW_REQUIRED":
		return "pending"
	default:
		return "none"
	}
}

func reviewQueueStatusLabel(pr PullRequest) string {
	if !pr.ViewerReviewAt.IsZero() && pr.LatestReviewRequestAt.After(pr.ViewerReviewAt) {
		return "review"
	}

	switch pr.ViewerReviewState {
	case "APPROVED":
		return "approved"
	case "COMMENTED":
		return "commented"
	case "CHANGES_REQUESTED":
		return "changes"
	}

	switch pr.ReviewDecision {
	case "CHANGES_REQUESTED":
		return "changes"
	case "APPROVED":
		return "settled"
	default:
		return "review"
	}
}

// MyPRsToRows converts a slice of PullRequests (My PRs tab) into table rows.
func MyPRsToRows(prs []PullRequest) []table.Row {
	rows := make([]table.Row, len(prs))
	for i, pr := range prs {
		rows[i] = table.Row{
			draftLabel(pr.IsDraft),
			pr.RepoShort(),
			pr.Title,
			pr.Age(),
			reviewDecisionLabel(pr.ReviewDecision),
		}
	}
	return rows
}

var styleBase = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(ui.ColorBorder)

var (
	styleError = lipgloss.NewStyle().Foreground(ui.ColorError)
)

var (
	activeTabStyle = lipgloss.NewStyle().
			Foreground(ui.ColorAccent).
			Background(ui.ColorSelected).
			Bold(true).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(ui.ColorText).
				Background(ui.ColorElevated).
				Padding(0, 2)

	tabSepStyle = lipgloss.NewStyle().
			Foreground(ui.ColorSubtleText)

	tabRowStyle = lipgloss.NewStyle()
)

func makeTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ui.ColorBorder).
		BorderBottom(true).
		Foreground(ui.ColorAccent).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(ui.ColorText).
		Background(ui.ColorSelected).
		Bold(true)
	return s
}

func emptyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ui.ColorSubtleText)
}

func tabBorderLine(width int) string {
	return lipgloss.NewStyle().
		Foreground(ui.ColorBorder).
		Render(strings.Repeat("━", width))
}
