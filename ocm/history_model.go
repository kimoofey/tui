package ocm

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/service"
)

type historyState struct {
	diagnosticsWarning          string
	viewportHeight              int
	backend                     service.HistoryBackend
	active, interacted, initial bool
	month                       time.Time
	all                         bool
	now                         func() time.Time
	report                      history.Report
	hasReport                   bool
	shownRange                  string
	queryID                     uint64
	querying                    bool
	queryErr                    error
	refreshing, pendingRefresh  bool
	refreshID                   uint64
	sourceErr                   error
	collection                  service.HistoryStatus
	offset                      int
	showHelp                    bool
}
type historyQueriedMsg struct {
	warning string
	id      uint64
	label   string
	report  history.Report
	err     error
}
type historySourceMsg struct {
	warning string
	id      uint64
	state   service.BrowserState
}

func utcMonth(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
func (h *historyState) query() tea.Cmd {
	h.queryID++
	id := h.queryID
	h.querying = true
	return h.queryCommand(id)
}
func (h *historyState) queryCommand(id uint64) tea.Cmd {
	backend := h.backend
	var period *history.Range
	label := "All time"
	if !h.all {
		period = &history.Range{Start: h.month.UnixMilli(), End: h.month.AddDate(0, 1, 0).UnixMilli()}
		label = h.month.Format("January 2006")
	}
	return func() tea.Msg {
		report, err := backend.Query(context.Background(), period)
		return historyQueriedMsg{id: id, label: label, report: report, err: err, warning: historyDiagnosticsWarning(backend)}
	}
}
func (h *historyState) refresh() tea.Cmd {
	if h.refreshing {
		h.pendingRefresh = true
		return nil
	}
	h.refreshing = true
	h.refreshID++
	id := h.refreshID
	return h.refreshCommand(id)
}
func (h *historyState) refreshCommand(id uint64) tea.Cmd {
	backend := h.backend
	return func() tea.Msg {
		state := backend.Refresh(context.Background())
		return historySourceMsg{id: id, state: state, warning: historyDiagnosticsWarning(backend)}
	}
}
func (m Model) browserUsable() bool { return m.app != nil && len(m.sessions) > 0 }
func (m Model) foregroundBusy() bool {
	return m.bulkDeleting || m.reloading || m.vacuuming || m.pruning || m.deletePhase == deletePending || m.vacuumPhase || m.prunePhase
}
func (m *Model) historyKey(msg tea.KeyPressMsg) tea.Cmd {
	h := &m.history
	h.interacted = true
	switch msg.String() {
	case "q", "ctrl+c":
		return tea.Quit
	case "h", "esc":
		if m.browserUsable() {
			h.active = false
		}
		return nil
	case "[", "]":
		if h.all {
			h.all = false
			h.month = utcMonth(h.now())
		} else {
			delta := -1
			if msg.String() == "]" {
				delta = 1
			}
			h.month = h.month.AddDate(0, delta, 0)
		}
		h.offset = 0
		return h.query()
	case "0":
		h.all = false
		h.month = utcMonth(h.now())
		h.offset = 0
		return h.query()
	case "a":
		h.all = true
		h.offset = 0
		return h.query()
	case "r":
		return tea.Batch(h.refresh(), h.query())
	case "up", "k":
		h.offset = max(0, h.offset-1)
	case "down", "j":
		h.offset = min(h.offset+1, max(0, len(h.lines(m.width))-max(1, m.height-3)))
	case "?":
		h.showHelp = !h.showHelp
	}
	return nil
}
func (m *Model) applyHistorySource(msg historySourceMsg) tea.Cmd {
	h := &m.history
	if msg.id != h.refreshID {
		return nil
	}
	if msg.warning != "" {
		h.diagnosticsWarning = msg.warning
	}
	h.refreshing = false
	h.sourceErr = msg.state.Err
	old := m.app
	if msg.state.Err != nil || msg.state.Snapshot == nil {
		m.app = nil
		m.capabilities = service.Capabilities{}
		m.updateKeys()
		h.active = true
	} else {
		m.app = msg.state.App
		m.applyDeleteData(reloadedMsg{snapshot: msg.state.Snapshot}, true)
		if len(m.sessions) == 0 {
			h.active = true
		} else if h.initial && !h.interacted {
			h.active = false
		}
	}
	h.initial = false
	var cmds []tea.Cmd
	if m.app != nil && m.app != old && m.app.HistoryEnabled() {
		h.collection = m.app.HistoryStatus()
		cmds = append(cmds, waitHistory(m.app))
	}
	if h.pendingRefresh {
		h.pendingRefresh = false
		cmds = append(cmds, h.refresh())
	}
	cmds = append(cmds, h.query())
	return tea.Batch(cmds...)
}
func safeLabel(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}
func historyAmount(v *float64) string {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return "unknown"
	}
	return fmt.Sprintf("$%.2f", *v)
}
func groupName(v *string) string {
	if v == nil {
		return "(unknown)"
	}
	return safeLabel(*v)
}
func (h historyState) lines(width int) []string {
	width = max(1, width)
	label := h.month.Format("January 2006")
	if h.all {
		label = "All time"
	}
	lines := []string{label + " · UTC", "Coverage limited to successfully observed usage."}
	if h.all || utcMonth(h.now()).Equal(h.month) {
		lines = append(lines, "Current month/day is partial.")
	}
	status := "Idle"
	if h.collection.Running || h.refreshing {
		status = "Collecting / refreshing…"
	}
	if h.sourceErr != nil {
		status = "Source unavailable: " + safeLabel(h.sourceErr.Error())
	} else if h.collection.Err != nil {
		status = "Capture failed / unsupported: " + safeLabel(h.collection.Err.Error())
	}
	lines = append(lines, status)
	if h.diagnosticsWarning != "" {
		lines = append(lines, h.diagnosticsWarning)
	}
	if h.querying {
		lines = append(lines, "Loading saved history…")
	}
	if h.queryErr != nil {
		lines = append(lines, "Query failed; previous results retained: "+safeLabel(h.queryErr.Error()))
	}
	if !h.hasReport {
		return wrapHistoryLines(append(lines, "No captured usage loaded"), width)
	}
	r := h.report
	if h.shownRange != label {
		lines = append(lines, "Showing previous results: "+h.shownRange)
	}
	var last int64
	stale, failed := 0, 0
	for _, s := range r.Sessions {
		last = max(last, s.Captured)
		if s.Status != "" {
			if strings.Contains(s.Status, "source records removed") {
				stale++
			} else {
				failed++
			}
		}
	}
	captured := "never"
	if len(r.Sessions) > 0 {
		captured = time.UnixMilli(last).UTC().Format("2006-01-02 15:04:05 UTC")
	}
	lines = append(lines, "Last capture: "+captured, fmt.Sprintf("Saved status: %d stale · %d failed", stale, failed))
	if stale > 0 {
		lines = append(lines, "Stale: source records removed or known fields lost; saved usage retained.")
	}
	if r.ApproximateV1 {
		lines = append(lines, "Approximate: V1 forked sessions may include copied usage.")
	}
	if len(r.Sessions) == 0 {
		lines = append(lines, "No captured usage")
	} else if r.Rows == 0 {
		lines = append(lines, "No captured usage in this range")
	}
	lines = append(lines, "Recorded total: "+historyAmount(r.Cost), fmt.Sprintf("Unknown: %d cost · %d provider · %d model", r.UnknownCost, r.UnknownProvider, r.UnknownModel), fmt.Sprintf("Unbucketed timestamps: %d · %s (all-time only)", r.Unbucketed, historyAmount(r.UnbucketedCost)), "", "Daily recorded USD")
	maximum, allMaximum := 0.0, 0.0
	row := 0
	for _, line := range lines {
		row += len(strings.Split(lipgloss.NewStyle().Width(width).Render(safeLabel(line)), "\n"))
	}
	for _, d := range r.Days {
		label := fmt.Sprintf("%s %s", d.UTC, historyAmount(d.Cost))
		if d.UnknownCost > 0 {
			label += fmt.Sprintf(" (%d unknown)", d.UnknownCost)
		}
		rows := len(strings.Split(lipgloss.NewStyle().Width(width).Render(label), "\n"))
		if d.Cost != nil && !math.IsInf(*d.Cost, 0) {
			allMaximum = math.Max(allMaximum, *d.Cost)
			if h.viewportHeight <= 0 || (row < h.offset+max(1, h.viewportHeight-2) && row+rows > h.offset) {
				maximum = math.Max(maximum, *d.Cost)
			}
		}
		row += rows
	}
	if maximum == 0 {
		maximum = allMaximum
	}
	barWidth := max(0, min(40, width-28))
	for _, d := range r.Days {
		amount := historyAmount(d.Cost)
		bar := ""
		if maximum > 0 && d.Cost != nil && *d.Cost > 0 {
			n := int((*d.Cost / maximum) * float64(barWidth))
			bar = strings.Repeat("█", max(0, min(n, barWidth)))
		}
		row := fmt.Sprintf("%s %s %s", d.UTC, amount, bar)
		if d.UnknownCost > 0 {
			row += fmt.Sprintf(" (%d unknown)", d.UnknownCost)
		}
		lines = append(lines, row)
	}
	lines = append(lines, "", "Provider / model totals")
	for _, model := range r.Models {
		lines = append(lines, fmt.Sprintf("%s · %s  %s (%d unknown costs)", groupName(model.Provider), groupName(model.Name), historyAmount(model.Cost), model.UnknownCost))
	}
	if h.showHelp {
		lines = append(lines, "", "[ / ] previous / next UTC month · 0 current month · a all time", "r refresh source and archive · ↑/↓ or j/k scroll", "esc / h back to browser when available · ? help · q / ctrl+c quit")
	}
	// Wrap metadata rather than truncate it: unknown/error/quality information
	// remains accessible by scrolling on narrow terminals.
	return wrapHistoryLines(lines, width)
}
func wrapHistoryLines(lines []string, width int) []string {
	var wrapped []string
	for _, line := range lines {
		parts := strings.Split(lipgloss.NewStyle().Width(width).Render(safeLabel(line)), "\n")
		wrapped = append(wrapped, parts...)
	}
	return wrapped
}
func (m Model) historyView() tea.View {
	width := max(1, m.width)
	height := max(1, m.height)
	title := lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true).Render("Recorded USD (approximate)")
	lines := m.history.lines(width)
	bodyHeight := max(0, height-2)
	start := min(m.history.offset, max(0, len(lines)-bodyHeight))
	end := min(len(lines), start+bodyHeight)
	body := lipgloss.NewStyle().Foreground(ui.ColorText).Render(strings.Join(lines[start:end], "\n"))
	footer := lipgloss.NewStyle().Foreground(ui.ColorSubtleText).Render("[ ] month · a all · r refresh · h back · ? help · q quit")
	content := title
	if height > 1 {
		content += "\n" + body
	}
	if height > 2 {
		content += "\n" + footer
	}
	// Constrain both axes even on very small terminals.
	content = lipgloss.NewStyle().MaxWidth(width).MaxHeight(height).Render(content)
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func historyDiagnosticsWarning(source any) string {
	if d, ok := source.(interface{ DiagnosticsWarning() string }); ok {
		return d.DiagnosticsWarning()
	}
	return ""
}
