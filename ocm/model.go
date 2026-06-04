package ocm

import (
	"fmt"
	"os"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm/db"
)

// keyMap defines all keybindings and their short/full help text.
type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	PgUp   key.Binding
	PgDn   key.Binding
	Space  key.Binding
	Esc    key.Binding
	Enter  key.Binding
	Delete key.Binding
	Vacuum key.Binding
	Prune  key.Binding
	Help   key.Binding
	Quit   key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Space, k.Delete, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PgUp, k.PgDn},
		{k.Enter, k.Space, k.Esc, k.Delete},
		{k.Vacuum, k.Prune, k.Help, k.Quit},
	}
}

// defaultKeys is the keybinding set used across the app.
var defaultKeys = keyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	PgUp:   key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("pgup/b", "page up")),
	PgDn:   key.NewBinding(key.WithKeys("pgdown", "f"), key.WithHelp("pgdn/f", "page down")),
	Space:  key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "mark")),
	Esc:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Vacuum: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "vacuum db")),
	Prune:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prune orphans")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more keys")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

func helpStyles() help.Styles {
	return ui.HelpStyles()
}

// Column widths (characters).
const (
	colWidthMarker = 1
	colWidthDate   = 13
	colWidthCost   = 7 // "$999.99" fits in 7

	minDirWidth   = 15
	minTitleWidth = 20

	// dirFrac is the fraction of flexible space given to the directory column.
	// The rest goes to title. At 120-wide this yields ~28 dir / ~44 title.
	dirFrac = 0.40

	numColumns    = 6                          // marker + directory + title + cost + created + updated
	cellPaddingLR = 2                          // table Cell style Padding(0,1): 1 left + 1 right per cell
	tableCellPad  = numColumns * cellPaddingLR // total horizontal padding added by the table renderer

	borderWidth = 2 // left + right border from styleBase (1 char each side)

	// layoutOverheadBase is the fixed chrome rows: title(1) + border-top(1) + border-bottom(1).
	// table.SetHeight includes the table header row internally.
	layoutOverheadBase = 3

	minReclaimableBytes int64 = 1 << 20
	mouseScrollLines          = 3
)

var (
	styleBase = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ui.ColorMuted)

	styleSuccess = lipgloss.NewStyle().
			Foreground(ui.ColorSuccess)

	styleWarning = lipgloss.NewStyle().
			Foreground(ui.ColorWarning)

	styleError = lipgloss.NewStyle().
			Foreground(ui.ColorError)
)

// vacuumedMsg is sent back after a VACUUM completes.
type vacuumedMsg struct {
	dbUsed      int64
	dbTotal     int64
	beforeTotal int64 // dbTotal before VACUUM, for the "X → Y" status message
	err         error
}

// prunedMsg is sent back after orphaned session-diff files are deleted.
type prunedMsg struct {
	count int
	bytes int64
	err   error
}

// openedMsg is sent back to the model after attempting to open a session.
type openedMsg struct {
	sessionID string
	err       error
}

// deletedMsg is sent back to the model after attempting to delete a session.
type deletedMsg struct {
	sessionID string
	newTotal  int
	err       error
}

// bulkDeletedMsg is sent back after a bulk delete completes.
type bulkDeletedMsg struct {
	count    int
	sessions []db.Session
	newTotal int
	err      error
}

// deleteState tracks the two-press delete confirmation flow.
type deleteState int

const (
	deleteNone    deleteState = iota
	deletePending             // first d pressed — waiting for confirmation
)

// Model is the bubbletea model for the session browser.
type Model struct {
	table           table.Model
	sessions        []db.Session
	selected        map[string]bool
	totalSessions   int
	dbUsed          int64
	dbTotal         int64
	costPeriod      CostPeriod
	periodCost      float64
	dbPath          string
	rootOnly        bool
	homeDir         string
	width           int
	height          int
	status          string
	statusOK        bool
	deletePhase     deleteState
	deleteTarget    int
	bulkDeleting    bool
	bulkDeleteCount int
	vacuumPhase     bool
	vacuuming       bool
	orphanCount     int
	orphanBytes     int64
	prunePhase      bool
	pruning         bool
	help            help.Model
	keys            keyMap
}

// currentOverhead returns the total terminal rows consumed by chrome (title,
// borders, footer). Dynamic because the footer expands when full help is shown.
func (m Model) currentOverhead() int {
	if m.help.ShowAll {
		// base(3) + status line(1) + full help rows (derived from longest FullHelp column)
		fullHelpRows := 0
		for _, col := range m.keys.FullHelp() {
			if len(col) > fullHelpRows {
				fullHelpRows = len(col)
			}
		}
		return layoutOverheadBase + 1 + fullHelpRows
	}
	// base(3) + single footer line combining help + inline status(1)
	return layoutOverheadBase + 1
}

type Options struct {
	Sessions    []db.Session
	TotalCount  int
	DBUsed      int64
	DBTotal     int64
	CostPeriod  CostPeriod
	PeriodCost  float64
	OrphanCount int
	OrphanBytes int64
	DBPath      string
	RootOnly    bool
}

// New creates a Model populated with the given sessions.
func New(opts Options, termWidth, termHeight int) Model {
	selected := make(map[string]bool)
	homeDir, _ := os.UserHomeDir()
	cols, rows := buildTable(opts.Sessions, selected, termWidth-borderWidth, homeDir)

	initialTableHeight := max(termHeight-(layoutOverheadBase+1), 1)

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(initialTableHeight),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ui.ColorMuted).
		BorderBottom(true).
		Bold(true).
		Foreground(ui.ColorAccent)
	s.Selected = s.Selected.
		Foreground(ui.ColorText).
		Background(ui.ColorSelected).
		Bold(true)
	t.SetStyles(s)

	h := help.New()
	h.Styles = helpStyles()
	h.SetWidth(termWidth - 2)

	return Model{
		table:         t,
		sessions:      opts.Sessions,
		selected:      selected,
		totalSessions: opts.TotalCount,
		dbUsed:        opts.DBUsed,
		dbTotal:       opts.DBTotal,
		costPeriod:    opts.CostPeriod,
		periodCost:    opts.PeriodCost,
		orphanCount:   opts.OrphanCount,
		orphanBytes:   opts.OrphanBytes,
		dbPath:        opts.DBPath,
		rootOnly:      opts.RootOnly,
		homeDir:       homeDir,
		width:         termWidth,
		height:        termHeight,
		help:          h,
		keys:          defaultKeys,
	}
}

// buildTable converts sessions into bubbles/table columns + rows.
// The directory and title columns share whatever space is left after the fixed
// columns, with directory getting dirFrac of that flexible budget.
func buildTable(sessions []db.Session, selected map[string]bool, termWidth int, homeDir string) ([]table.Column, []table.Row) {
	fixedUsed := colWidthMarker + colWidthCost + colWidthDate*2 + tableCellPad
	flex := max(termWidth-fixedUsed, minDirWidth+minTitleWidth)
	dirWidth := max(int(float64(flex)*dirFrac), minDirWidth)
	titleWidth := max(flex-dirWidth, minTitleWidth)

	cols := []table.Column{
		{Title: " ", Width: colWidthMarker},
		{Title: "Directory", Width: dirWidth},
		{Title: "Title", Width: titleWidth},
		{Title: "Cost", Width: colWidthCost},
		{Title: "Created", Width: colWidthDate},
		{Title: "Updated", Width: colWidthDate},
	}

	rows := make([]table.Row, len(sessions))
	for i, s := range sessions {
		marker := " "
		if selected[s.ID] {
			marker = "●"
		}
		rows[i] = table.Row{
			marker,
			ui.Truncate(shortenHome(s.Directory, homeDir), dirWidth),
			s.Title,
			formatCost(s.Cost),
			formatDate(s.Created),
			formatDate(s.Updated),
		}
	}

	return cols, rows
}

func (m Model) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width - 2)
		cols, rows := buildTable(m.sessions, m.selected, m.width-borderWidth, m.homeDir)
		m.table.SetColumns(cols)
		m.table.SetRows(rows)
		m.table.SetHeight(m.height - m.currentOverhead())
		m.table.SetWidth(m.width - borderWidth)

	case tea.MouseWheelMsg:
		if m.bulkDeleting || m.vacuuming || m.pruning {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseWheelUp:
			m.table.MoveUp(mouseScrollLines)
		case tea.MouseWheelDown:
			m.table.MoveDown(mouseScrollLines)
		}
		return m, nil // already handled — don't forward to table.Update below

	case tea.KeyPressMsg:
		// Lock all input during bulk delete, VACUUM, or prune.
		if m.bulkDeleting || m.vacuuming || m.pruning {
			return m, nil
		}

		m.status = ""

		// During delete confirmation, lock all input — no table forwarding.
		if m.deletePhase == deletePending {
			if msg.String() == "d" {
				m.deletePhase = deleteNone
				if len(m.selected) > 0 {
					ids := make([]string, 0, len(m.selected))
					for id := range m.selected {
						ids = append(ids, id)
					}
					m.bulkDeleting = true
					m.bulkDeleteCount = len(ids)
					return m, bulkDeleteSessions(ids, m.dbPath, m.rootOnly)
				}
				return m, deleteSession(m.sessions[m.deleteTarget].ID, m.dbPath)
			}
			m.deletePhase = deleteNone
			return m, nil
		}

		// During vacuum confirmation, v confirms, any other key cancels.
		if m.vacuumPhase {
			if msg.String() == "v" {
				m.vacuumPhase = false
				m.vacuuming = true
				return m, vacuumDB(m.dbPath, m.dbTotal)
			}
			m.vacuumPhase = false
			return m, nil
		}

		// During prune confirmation, p confirms, any other key cancels.
		if m.prunePhase {
			if msg.String() == "p" {
				m.prunePhase = false
				m.pruning = true
				return m, pruneOrphans(m.dbPath)
			}
			m.prunePhase = false
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.help.ShowAll {
				m.help.ShowAll = false
				m.table.SetHeight(m.height - m.currentOverhead())
				return m, nil
			}
			if len(m.selected) > 0 {
				m.selected = make(map[string]bool)
				m.rebuildRows()
			}
			return m, nil
		case "enter":
			if len(m.sessions) == 0 {
				break
			}
			idx := m.table.Cursor()
			if idx < 0 || idx >= len(m.sessions) {
				break
			}
			sid := m.sessions[idx].ID
			return m, openSession(sid)
		case "d":
			if len(m.sessions) == 0 {
				break
			}
			if len(m.selected) > 0 {
				m.deletePhase = deletePending
				m.deleteTarget = -1
			} else {
				idx := m.table.Cursor()
				if idx < 0 || idx >= len(m.sessions) {
					break
				}
				m.deletePhase = deletePending
				m.deleteTarget = idx
			}
			return m, nil // don't forward to table
		case "v":
			if m.dbTotal-m.dbUsed < minReclaimableBytes {
				m.status = "nothing to vacuum"
				m.statusOK = false
				return m, nil
			}
			m.vacuumPhase = true
			return m, nil
		case "p":
			if m.orphanCount == 0 {
				m.status = "nothing to prune"
				m.statusOK = false
				return m, nil
			}
			m.prunePhase = true
			return m, nil
		case "?":
			m.help.ShowAll = !m.help.ShowAll
			m.table.SetHeight(m.height - m.currentOverhead())
			return m, nil
		}

		// Space is handled via key.Matches to avoid the raw string ambiguity
		// (" " vs "space") and to ensure it is consumed before the table sees it.
		if key.Matches(msg, m.keys.Space) {
			if len(m.sessions) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.sessions) {
					id := m.sessions[idx].ID
					if m.selected[id] {
						delete(m.selected, id)
					} else {
						m.selected[id] = true
					}
					m.rebuildRows()
				}
			}
			return m, nil
		}

	case deletedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("error deleting session: %s", msg.err)
			m.statusOK = false
		} else {
			for i, s := range m.sessions {
				if s.ID == msg.sessionID {
					m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
					break
				}
			}
			if msg.newTotal > 0 {
				m.totalSessions = msg.newTotal
			} else {
				m.totalSessions--
			}
			m.rebuildRows()
			if m.table.Cursor() >= len(m.sessions) && len(m.sessions) > 0 {
				m.table.SetCursor(len(m.sessions) - 1)
			}
			m.status = fmt.Sprintf("deleted %s", msg.sessionID)
			m.statusOK = true
		}

	case bulkDeletedMsg:
		m.bulkDeleting = false
		if msg.err != nil {
			m.status = fmt.Sprintf("error during bulk delete: %s", msg.err)
			m.statusOK = false
		} else {
			m.sessions = msg.sessions
			m.totalSessions = msg.newTotal
			m.selected = make(map[string]bool)
			m.rebuildRows()
			if m.table.Cursor() >= len(m.sessions) && len(m.sessions) > 0 {
				m.table.SetCursor(len(m.sessions) - 1)
			}
			if msg.count == 1 {
				m.status = "deleted 1 session"
			} else {
				m.status = fmt.Sprintf("deleted %d sessions", msg.count)
			}
			m.statusOK = true
		}

	case openedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("error opening %s: %s", msg.sessionID, msg.err)
			m.statusOK = false
		} else {
			m.status = fmt.Sprintf("opened %s in new tab", msg.sessionID)
			m.statusOK = true
		}

	case vacuumedMsg:
		m.vacuuming = false
		if msg.err != nil {
			m.status = fmt.Sprintf("vacuum failed: %s", msg.err)
			m.statusOK = false
		} else {
			m.dbUsed = msg.dbUsed
			m.dbTotal = msg.dbTotal
			m.status = fmt.Sprintf("vacuumed: %s → %s", formatSize(msg.beforeTotal), formatSize(msg.dbTotal))
			m.statusOK = true
		}

	case prunedMsg:
		m.pruning = false
		if msg.err != nil {
			m.status = fmt.Sprintf("prune failed: %s", msg.err)
			m.statusOK = false
		} else {
			m.orphanCount = 0
			m.orphanBytes = 0
			m.status = fmt.Sprintf("pruned %d files (%s)", msg.count, formatSize(msg.bytes))
			m.statusOK = true
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	titleBar := m.renderTitleBar()
	var statusText string
	switch {
	case m.vacuuming:
		statusText = styleWarning.Render("vacuuming...")
	case m.vacuumPhase:
		reclaimable := m.dbTotal - m.dbUsed
		statusText = styleWarning.Render(fmt.Sprintf("v to vacuum · reclaim ~%s · any key to cancel", formatSize(reclaimable)))
	case m.pruning:
		statusText = styleWarning.Render("pruning...")
	case m.prunePhase:
		statusText = styleWarning.Render(fmt.Sprintf("p to prune %d orphans (~%s) · any key to cancel", m.orphanCount, formatSize(m.orphanBytes)))
	case m.bulkDeleting:
		statusText = styleError.Render(fmt.Sprintf("deleting %d sessions...", m.bulkDeleteCount))
	case m.deletePhase == deletePending && len(m.selected) > 0:
		statusText = styleError.Render(fmt.Sprintf("d to confirm delete %d sessions  ·  any key to cancel", len(m.selected)))
	case m.deletePhase == deletePending:
		statusText = styleError.Render("d to confirm delete  ·  any key to cancel")
	case m.status != "" && m.statusOK:
		statusText = styleSuccess.Render("✓ " + m.status)
	case m.status != "":
		statusText = styleError.Render("✗ " + m.status)
	}

	var footer string
	helpRendered := m.help.View(m.keys)
	innerWidth := m.width - 2 // -2 for Padding(0,1) on each side
	if m.help.ShowAll {
		var statusLine string
		if statusText != "" {
			statusLine = statusBarStyle().Render(
				lipgloss.PlaceHorizontal(innerWidth, lipgloss.Right, statusText))
		} else {
			statusLine = statusBarStyle().Render("")
		}
		footer = lipgloss.JoinVertical(lipgloss.Left,
			statusLine,
			statusBarStyle().Render(helpRendered),
		)
	} else {
		var footerContent string
		if statusText != "" {
			rightAligned := lipgloss.PlaceHorizontal(innerWidth-lipgloss.Width(helpRendered), lipgloss.Right, statusText)
			footerContent = helpRendered + rightAligned
		} else {
			footerContent = helpRendered
		}
		footer = statusBarStyle().Render(footerContent)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleBar,
		styleBase.Render(m.table.View()),
		footer,
	)

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// rebuildRows regenerates table rows from current session + selection state.
func (m *Model) rebuildRows() {
	_, rows := buildTable(m.sessions, m.selected, m.width-borderWidth, m.homeDir)
	m.table.SetRows(rows)
}

func titleBarStyle() lipgloss.Style {
	return ui.TitleBarStyle()
}

func statusBarStyle() lipgloss.Style {
	return ui.StatusBarStyle()
}

// renderTitleBar renders the top bar: "ocm" on the left, session count + db size on the right.
func (m Model) renderTitleBar() string {
	left := "ocm"

	n := len(m.sessions)
	var sessionLabel string
	switch {
	case n == 0:
		sessionLabel = "no sessions"
	case n == 1 && n == m.totalSessions:
		sessionLabel = "1 session"
	case n == m.totalSessions:
		sessionLabel = fmt.Sprintf("%d sessions", n)
	case n == 1:
		sessionLabel = fmt.Sprintf("1 / %d sessions", m.totalSessions)
	default:
		sessionLabel = fmt.Sprintf("%d / %d sessions", n, m.totalSessions)
	}

	var sizeLabel string
	usedStr := formatSize(m.dbUsed)
	totalStr := formatSize(m.dbTotal)
	if usedStr == totalStr {
		sizeLabel = totalStr
	} else {
		sizeLabel = usedStr + " / " + totalStr
	}
	right := sessionLabel + "  ·  " + sizeLabel
	if m.costPeriod != "" {
		right += "  ·  " + string(m.costPeriod) + ": " + fmt.Sprintf("$%.2f", m.periodCost)
	}

	styledLeft := lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true).Render(left)
	styledRight := lipgloss.NewStyle().Foreground(ui.ColorText).Render(right)

	innerWidth := m.width - 2
	rightAligned := lipgloss.PlaceHorizontal(innerWidth-lipgloss.Width(left), lipgloss.Right, styledRight)
	return titleBarStyle().Render(styledLeft + rightAligned)
}
