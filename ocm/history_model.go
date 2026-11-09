package ocm

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/service"
)

type historyState struct {
	loading                     startupLoadingState
	setup                       historyImportState
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
func (m Model) loadingSessions() bool {
	return m.history.backend != nil && m.history.initial && !m.history.active && !m.browserUsable()
}

func (m Model) sessionsLoadingView() tea.View {
	content := ""
	if m.history.loading.visible {
		width, height := max(1, m.width), max(1, m.height)
		content = m.history.loading.spinner.View() + " " + lipgloss.NewStyle().Foreground(ui.ColorText).Render("Loading sessions…")
		content = lipgloss.NewStyle().MaxWidth(width).MaxHeight(height).Render(content)
		content = lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) foregroundBusy() bool {
	return m.bulkDeleting || m.reloading || m.vacuuming || m.pruning || m.deletePhase == deletePending || m.vacuumPhase || m.prunePhase
}

func (m Model) historyBodySize() (int, int) {
	footerHeight := lipgloss.Height(m.historyFooter())
	if m.width < 5 || m.height < 5 {
		return max(1, m.width), max(0, m.height-1-footerHeight)
	}
	return m.width - 4, max(1, m.height-3-footerHeight)
}

func (m Model) historyMaxOffset() int {
	width, height := m.historyBodySize()
	return max(0, len(m.history.lines(width))-max(1, height))
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
	case "[", "]", "left", "right":
		if h.all {
			h.all = false
			h.month = utcMonth(h.now())
		} else {
			delta := -1
			if msg.String() == "]" || msg.String() == "right" {
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
		if h.setup.pending && m.app != nil {
			h.setup.dialog = "prompt"
			return nil
		}
		return tea.Batch(h.refresh(), h.query())
	case "up", "k":
		h.offset = max(0, h.offset-1)
	case "down", "j":
		h.offset = min(h.offset+1, m.historyMaxOffset())
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
	if m.loadingSessions() && h.loading.visible && !h.loading.minimumElapsed {
		// Keep the first result until the visible loader has had time to settle.
		h.loading.pending = &msg
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
	if msg.state.Err == nil {
		h.setup.pending = msg.state.ImportPending
		if h.setup.pending && !h.setup.offered {
			h.setup.offered = true
			h.setup.dialog = "prompt"
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
	lines := []string{label + " · UTC", "← previous month · next month →"}
	if h.setup.pending {
		lines = append(lines, "Initial import incomplete; coverage may be partial. Press r to start/retry.")
	}
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
	lines = append(lines, "", "Recorded spending", historyAmount(r.Cost),
		fmt.Sprintf("Unknown: %d cost · %d provider · %d model", r.UnknownCost, r.UnknownProvider, r.UnknownModel), "", "Provider / model totals")
	models := append([]history.Model(nil), r.Models...)
	sort.SliceStable(models, func(i, j int) bool {
		if models[i].Cost == nil || math.IsNaN(*models[i].Cost) || math.IsInf(*models[i].Cost, 0) {
			return false
		}
		if models[j].Cost == nil || math.IsNaN(*models[j].Cost) || math.IsInf(*models[j].Cost, 0) {
			return true
		}
		return *models[i].Cost > *models[j].Cost
	})
	maximumModel := 0.0
	for _, model := range models {
		if model.Cost != nil && !math.IsNaN(*model.Cost) && !math.IsInf(*model.Cost, 0) {
			maximumModel = math.Max(maximumModel, *model.Cost)
		}
	}
	for _, model := range models {
		bar := ""
		if maximumModel > 0 && model.Cost != nil && !math.IsNaN(*model.Cost) && !math.IsInf(*model.Cost, 0) && *model.Cost > 0 {
			bar = strings.Repeat("█", int(*model.Cost/maximumModel*float64(max(0, min(20, width-30)))))
		}
		lines = append(lines, fmt.Sprintf("%s · %s  %s %s (%d unknown costs)", groupName(model.Provider), groupName(model.Name), historyAmount(model.Cost), bar, model.UnknownCost))
	}
	lines = append(lines, "", "Coverage limited to successfully observed usage.", "Last capture: "+captured, fmt.Sprintf("Saved status: %d stale · %d failed", stale, failed))
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
	lines = append(lines, fmt.Sprintf("Unbucketed timestamps: %d · %s (all-time only)", r.Unbucketed, historyAmount(r.UnbucketedCost)))
	if !h.all && h.shownRange != "All time" {
		lines = append(lines, "", "Daily recorded USD")
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
				if h.viewportHeight <= 0 || (row < h.offset+max(1, h.viewportHeight-4) && row+rows > h.offset) {
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
func (m Model) historyFooter() string {
	help := m.help
	help.SetWidth(max(1, m.width-2))
	month := key.NewBinding(key.WithKeys("left", "right", "[", "]"), key.WithHelp("←/→, [/]", "change month"))
	current := key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "current month"))
	all := key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all time"))
	refresh := key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))
	scroll := key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓, j/k", "scroll"))
	back := key.NewBinding(key.WithKeys("h", "esc"), key.WithHelp("h/esc", "back"))
	toggle := key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help"))
	quit := key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "quit"))
	var footer string
	if m.history.showHelp {
		footer = lipgloss.JoinVertical(lipgloss.Left, ui.StatusBarStyle().Render(""), ui.StatusBarStyle().Render(help.FullHelpView([][]key.Binding{{month, current, all, refresh}, {scroll, back, toggle, quit}})))
	} else {
		footer = ui.StatusBarStyle().Render(help.ShortHelpView([]key.Binding{month, current, all, refresh, back, toggle}))
	}
	maxHeight := max(1, m.height-4)
	return lipgloss.NewStyle().MaxHeight(maxHeight).Render(footer)
}

func (m Model) historyView() tea.View {
	width := max(1, m.width)
	height := max(1, m.height)
	title := ui.TitleBarStyle().Render(lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true).Render("ocm") + "  " + lipgloss.NewStyle().Foreground(ui.ColorText).Render("History · Recorded USD (approximate)"))
	bodyWidth, bodyHeight := m.historyBodySize()
	lines := m.history.lines(bodyWidth)
	start := min(m.history.offset, max(0, len(lines)-bodyHeight))
	end := min(len(lines), start+bodyHeight)
	var rendered []string
	for i := start; i < end; i++ {
		style := lipgloss.NewStyle().Foreground(ui.ColorText)
		if i > 0 && lines[i-1] == "Recorded spending" {
			style = style.Foreground(ui.ColorAccent).Bold(true)
		}
		rendered = append(rendered, style.Render(lines[i]))
	}
	for len(rendered) < bodyHeight {
		rendered = append(rendered, "")
	}
	body := strings.Join(rendered, "\n")
	if width >= 5 && height >= 5 {
		body = styleBase.Width(width).Padding(0, 1).Render(body)
	}
	footer := m.historyFooter()
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
