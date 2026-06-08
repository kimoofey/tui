package prq

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"github.com/kimoofey/tui/internal/ui"
)

const (
	widthSource    = 8
	widthRepo      = 16
	widthAuthor    = 27
	widthPending   = 7
	widthApprovals = 9
	widthEstimate  = 8

	minTitleWidth = 12

	numCols = 7

	tableFixedWidth = widthSource + widthRepo +
		widthAuthor + widthPending + widthApprovals + widthEstimate + numCols*2

	borderWidth = 2
)

func titleWidth(termWidth int) int {
	w := max(termWidth-tableFixedWidth-2, minTitleWidth)
	return w
}

func makeColumns(termWidth int) []table.Column {
	return []table.Column{
		{Title: "Source", Width: widthSource},
		{Title: "Repo", Width: widthRepo},
		{Title: "Title", Width: titleWidth(termWidth)},
		{Title: "Author", Width: widthAuthor},
		{Title: "Pending", Width: widthPending},
		{Title: "Time", Width: widthEstimate},
		{Title: "Approvals", Width: widthApprovals},
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
func PRsToRows(prs []PullRequest, minApprovals int, estimateBuckets []int) []table.Row {
	rows := make([]table.Row, len(prs))
	for i, pr := range prs {
		approvalStr := fmt.Sprintf("%d/%d", pr.Approvals, minApprovals)
		if pr.Approvals >= minApprovals {
			approvalStr += " ✓"
		}
		rows[i] = table.Row{
			bucketLabel(pr.Bucket),
			ui.Truncate(pr.RepoShort(), widthRepo),
			pr.Title,
			"@" + ui.Truncate(pr.Author, widthAuthor-1),
			waitCell(pr),
			estimateCell(pr, estimateBuckets),
			approvalStr,
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

	myTableFixedWidth = widthSource + widthRepo + widthPending + widthMyReview + numMyCols*2
)

func myTitleWidth(termWidth int) int {
	w := max(termWidth-myTableFixedWidth-2, minTitleWidth)
	return w
}

func makeMyColumns(termWidth int) []table.Column {
	return []table.Column{
		{Title: "Status", Width: widthSource},
		{Title: "Repo", Width: widthRepo},
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

// MyPRsToRows converts a slice of PullRequests (My PRs tab) into table rows.
func MyPRsToRows(prs []PullRequest) []table.Row {
	rows := make([]table.Row, len(prs))
	for i, pr := range prs {
		rows[i] = table.Row{
			draftLabel(pr.IsDraft),
			ui.Truncate(pr.RepoShort(), widthRepo),
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
