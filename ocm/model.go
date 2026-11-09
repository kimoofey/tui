package ocm

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm/service"
	"github.com/kimoofey/tui/ocm/session"
)

type keyMap struct {
	History key.Binding
	Up      key.Binding
	Down    key.Binding
	PgUp    key.Binding
	PgDn    key.Binding
	Space   key.Binding
	Esc     key.Binding
	Enter   key.Binding
	Reload  key.Binding
	Delete  key.Binding
	Vacuum  key.Binding
	Prune   key.Binding
	Help    key.Binding
	Quit    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Space, k.Reload, k.Delete, k.History, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PgUp, k.PgDn},
		{k.Enter, k.Space, k.Esc, k.Reload, k.Delete},
		{k.Vacuum, k.Prune, k.History, k.Help, k.Quit},
	}
}

var defaultKeys = keyMap{
	History: key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "history")),
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	PgUp:    key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("pgup/b", "page up")),
	PgDn:    key.NewBinding(key.WithKeys("pgdown", "f"), key.WithHelp("pgdn/f", "page down")),
	Space:   key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "mark")),
	Esc:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	Reload:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload")),
	Delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Vacuum:  key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "vacuum db")),
	Prune:   key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prune orphans")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more keys")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
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
)

var (
	styleBase = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ui.ColorBorder)

	styleSuccess = lipgloss.NewStyle().
			Foreground(ui.ColorSuccess)

	styleWarning = lipgloss.NewStyle().
			Foreground(ui.ColorWarning)

	styleError = lipgloss.NewStyle().
			Foreground(ui.ColorError)
)

type vacuumedMsg struct {
	dbUsed      int64
	dbTotal     int64
	beforeTotal int64 // dbTotal before VACUUM, for the "X → Y" status message
	err         error
}

type prunedMsg struct {
	count int
	bytes int64
	err   error
}

type openedMsg struct {
	sessionID session.Ref
	err       error
}

type deletedMsg struct {
	sessionID       session.Ref
	acknowledgement bool
	data            *reloadedMsg // nil for mock command acknowledgements
	err             error
}

type bulkDeletedMsg struct {
	count           int
	ids             []session.Ref
	acknowledgement bool
	data            *reloadedMsg // nil for mock command acknowledgements
	err             error
}

type reloadedMsg struct {
	snapshot *service.Snapshot
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
	history         historyState
	table           table.Model
	sessions        []session.Session
	selected        map[session.Ref]bool
	totalSessions   int
	dbUsed          int64
	dbTotal         int64
	costPeriod      service.CostPeriod
	periodCost      float64
	app             *service.Service
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
	reloading       bool
	vacuumPhase     bool
	vacuuming       bool
	orphanCount     int
	orphanBytes     int64
	prunePhase      bool
	pruning         bool
	help            help.Model
	keys            keyMap
	capabilities    service.Capabilities
	metricsKnown    bool
	diagnostics     []error
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
	History      service.HistoryBackend
	Now          func() time.Time
	Sessions     []session.Session
	TotalCount   int
	DBUsed       int64
	DBTotal      int64
	CostPeriod   service.CostPeriod
	PeriodCost   float64
	OrphanCount  int
	OrphanBytes  int64
	Service      *service.Service
	RootOnly     bool
	Capabilities service.Capabilities
	MetricsKnown bool
	Diagnostics  []error
	Close        func() error
}

// New creates a Model populated with the given sessions.
func New(opts Options, termWidth, termHeight int) Model {
	selected := make(map[session.Ref]bool)
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
		BorderForeground(ui.ColorBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(ui.ColorAccent)
	s.Selected = s.Selected.
		Foreground(ui.ColorText).
		Background(ui.ColorSelected).
		Bold(true)
	t.SetStyles(s)

	h := help.New()
	h.Styles = ui.HelpStyles()
	h.SetWidth(termWidth - 2)

	m := Model{
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
		app:           opts.Service,
		rootOnly:      opts.RootOnly,
		homeDir:       homeDir,
		width:         termWidth,
		height:        termHeight,
		help:          h,
		keys:          defaultKeys,
		capabilities:  opts.Capabilities,
		metricsKnown:  opts.MetricsKnown,
		diagnostics:   opts.Diagnostics,
	}
	if opts.History != nil {
		now := opts.Now
		if now == nil {
			now = time.Now
		}
		m.history = historyState{backend: opts.History, viewportHeight: termHeight, active: len(opts.Sessions) == 0, month: utcMonth(now()), now: now, initial: true, queryID: 1, querying: true, refreshID: 1, refreshing: true}
	}
	m.updateKeys()
	if len(m.diagnostics) > 0 {
		m.status = m.diagnostics[0].Error()
	}
	return m
}

// buildTable converts sessions into bubbles/table columns + rows.
// The directory and title columns share whatever space is left after the fixed
// columns, with directory getting dirFrac of that flexible budget.
func buildTable(sessions []session.Session, selected map[session.Ref]bool, termWidth int, homeDir string) ([]table.Column, []table.Row) {
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
		if selected[s.Ref] {
			marker = "●"
		}
		rows[i] = table.Row{
			marker,
			ui.Truncate(shortenHome(s.Directory, homeDir), dirWidth),
			s.Title,
			formatRecordedCost(s.Cost),
			formatDate(s.Created),
			formatDate(s.Updated),
		}
	}

	return cols, rows
}

func (m Model) Init() tea.Cmd {
	if m.history.backend != nil {
		return tea.Batch(tea.RequestBackgroundColor, m.history.queryCommand(m.history.queryID), m.history.refreshCommand(m.history.refreshID))
	}
	if m.app != nil && m.app.HistoryEnabled() {
		return tea.Batch(tea.RequestBackgroundColor, waitHistory(m.app))
	}
	return tea.RequestBackgroundColor
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.history.backend != nil {
		switch event := msg.(type) {
		case historyQueriedMsg:
			if event.id == m.history.queryID {
				if event.warning != "" {
					m.history.diagnosticsWarning = event.warning
				}
				m.history.querying = false
				m.history.queryErr = event.err
				if event.err == nil {
					m.history.report = event.report
					m.history.hasReport = true
					m.history.shownRange = event.label
				}
			}
			return m, nil
		case historySourceMsg:
			cmd := m.applyHistorySource(event)
			return m, cmd
		case tea.KeyPressMsg:
			if m.history.refreshing && !m.history.active {
				switch event.String() {
				case "d", "v", "p":
					return m, nil
				}
			}
			if event.String() == "h" && m.foregroundBusy() {
				return m, nil
			}
			if m.history.active {
				cmd := m.historyKey(event)
				return m, cmd
			}
			if event.String() == "h" && !m.foregroundBusy() {
				m.history.active = true
				m.history.interacted = true
				cmd := m.history.query()
				return m, cmd
			}
			if event.String() == "r" && !m.foregroundBusy() {
				cmd := tea.Batch(m.history.refresh(), m.history.query())
				return m, cmd
			}
		case tea.MouseWheelMsg:
			if m.history.active {
				delta := ui.MouseScrollLines
				if event.Button == tea.MouseWheelUp {
					delta = -delta
				}
				m.history.offset = max(0, min(m.history.offset+delta, max(0, len(m.history.lines(m.width))-max(1, m.height-2))))
				return m, nil
			}
		}
	}

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.history.viewportHeight = msg.Height
		m.help.SetWidth(msg.Width - 2)
		cols, rows := buildTable(m.sessions, m.selected, m.width-borderWidth, m.homeDir)
		m.table.SetColumns(cols)
		m.table.SetRows(rows)
		m.table.SetHeight(max(1, m.height-m.currentOverhead()))
		m.table.SetWidth(max(1, m.width-borderWidth))

	case tea.MouseWheelMsg:
		if m.bulkDeleting || m.reloading || m.vacuuming || m.pruning {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseWheelUp:
			m.table.MoveUp(ui.MouseScrollLines)
		case tea.MouseWheelDown:
			m.table.MoveDown(ui.MouseScrollLines)
		}
		return m, nil // already handled — don't forward to table.Update below

	case tea.KeyPressMsg:
		// Lock all input during bulk delete, VACUUM, or prune.
		if m.bulkDeleting || m.reloading || m.vacuuming || m.pruning {
			return m, nil
		}

		m.status = ""

		// During delete confirmation, lock all input — no table forwarding.
		if m.deletePhase == deletePending {
			if msg.String() == "d" {
				m.deletePhase = deleteNone
				if len(m.selected) > 0 {
					ids := make([]session.Ref, 0, len(m.selected))
					for id := range m.selected {
						ids = append(ids, id)
					}
					m.bulkDeleting = true
					m.bulkDeleteCount = len(ids)
					return m, bulkDeleteSessions(ids, m.app, m.rootOnly, m.costPeriod)
				}
				m.bulkDeleting = true
				m.bulkDeleteCount = 1
				return m, deleteSession(m.sessions[m.deleteTarget].Ref, m.app, m.rootOnly, m.costPeriod)
			}
			m.deletePhase = deleteNone
			return m, nil
		}

		// During vacuum confirmation, v confirms, any other key cancels.
		if m.vacuumPhase {
			if msg.String() == "v" {
				m.vacuumPhase = false
				m.vacuuming = true
				return m, vacuumDB(m.app, m.dbTotal)
			}
			m.vacuumPhase = false
			return m, nil
		}

		// During prune confirmation, p confirms, any other key cancels.
		if m.prunePhase {
			if msg.String() == "p" {
				m.prunePhase = false
				m.pruning = true
				return m, pruneOrphans(m.app)
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
				m.table.SetHeight(max(1, m.height-m.currentOverhead()))
				return m, nil
			}
			if len(m.selected) > 0 {
				m.selected = make(map[session.Ref]bool)
				m.rebuildRows()
			}
			return m, nil
		case "enter":
			if !m.capabilities.Resume || len(m.sessions) == 0 {
				break
			}
			idx := m.table.Cursor()
			if idx < 0 || idx >= len(m.sessions) {
				break
			}
			sid := m.sessions[idx].Ref
			return m, openSession(sid, m.app)
		case "d":
			if !m.capabilities.Delete {
				return m, nil
			}
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
		case "r":
			m.reloading = true
			return m, reloadData(m.app, m.rootOnly, m.costPeriod)
		case "v":
			if !m.capabilities.Vacuum.Available {
				return m, nil
			}
			if m.dbTotal-m.dbUsed < minReclaimableBytes {
				m.status = "nothing to vacuum"
				m.statusOK = false
				return m, nil
			}
			m.vacuumPhase = true
			return m, nil
		case "p":
			if !m.capabilities.Orphans.Available {
				m.status = "pruning unavailable: " + m.capabilities.Orphans.Reason
				m.statusOK = false
				return m, nil
			}
			if m.orphanCount == 0 {
				m.status = "nothing to prune"
				m.statusOK = false
				return m, nil
			}
			m.prunePhase = true
			return m, nil
		case "?":
			m.help.ShowAll = !m.help.ShowAll
			m.table.SetHeight(max(1, m.height-m.currentOverhead()))
			return m, nil
		}

		// Space is handled via key.Matches to avoid the raw string ambiguity
		// (" " vs "space") and to ensure it is consumed before the table sees it.
		if key.Matches(msg, m.keys.Space) {
			if len(m.sessions) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.sessions) {
					id := m.sessions[idx].Ref
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
		m.bulkDeleting = false
		if msg.data != nil && msg.data.err == nil && msg.data.snapshot != nil {
			m.applyDeleteData(*msg.data, msg.err != nil)
		} else if msg.acknowledgement && msg.data == nil && msg.err == nil {
			m.removeAcknowledgedSessions([]session.Ref{msg.sessionID})
		}
		if msg.err != nil {
			m.status = fmt.Sprintf("error deleting session: %s", msg.err)
			m.statusOK = false
		} else {
			m.status = fmt.Sprintf("deleted %s", msg.sessionID.ID)
			m.statusOK = true
		}

	case bulkDeletedMsg:
		m.bulkDeleting = false
		if msg.data != nil && msg.data.err == nil && msg.data.snapshot != nil {
			m.applyDeleteData(*msg.data, msg.err != nil)
		} else if msg.acknowledgement && msg.data == nil && msg.err == nil {
			m.removeAcknowledgedSessions(msg.ids)
		}
		if msg.err != nil {
			m.status = fmt.Sprintf("error during bulk delete: %s", msg.err)
			m.statusOK = false
		} else {
			if msg.count == 1 {
				m.status = "deleted 1 session"
			} else {
				m.status = fmt.Sprintf("deleted %d sessions", msg.count)
			}
			if m.app != nil && m.app.HistoryEnabled() {
				m.status = fmt.Sprintf("deleted %d session trees", msg.count)
			}
			m.statusOK = true
		}

	case historyCollectedMsg:
		if msg.app != nil && msg.app != m.app {
			return m, nil
		}
		if m.history.backend != nil {
			if msg.status.Revision < m.history.collection.Revision {
				return m, waitHistory(m.app)
			}
			if msg.warning != "" {
				m.history.diagnosticsWarning = msg.warning
			}
			m.history.collection = msg.status
			if m.app != nil {
				m.capabilities = m.app.Capabilities()
				m.updateKeys()
			}
			cmd := tea.Batch(waitHistory(m.app), m.history.query())
			return m, cmd
		}
		if m.app != nil {
			m.capabilities = m.app.Capabilities()
			m.updateKeys()
		}
		if msg.status.Err != nil {
			m.status = "history: " + msg.status.Err.Error()
			m.statusOK = false
		} else if strings.HasPrefix(m.status, "history:") {
			m.status = "history: collection complete"
			m.statusOK = true
		}
		return m, waitHistory(m.app)

	case reloadedMsg:
		m.reloading = false
		if msg.err == nil && msg.snapshot == nil {
			msg.err = fmt.Errorf("reload returned no snapshot")
		}
		if msg.err != nil {
			m.status = fmt.Sprintf("reload failed: %s", msg.err)
			m.statusOK = false
		} else {
			m.applyData(msg)
			m.status = "reloaded"
			if len(m.diagnostics) > 0 {
				m.status = m.diagnostics[0].Error()
			}
			m.statusOK = true
		}

	case openedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("error opening %s: %s", msg.sessionID.ID, msg.err)
			m.statusOK = false
		} else {
			m.status = fmt.Sprintf("opened %s in new tab", msg.sessionID.ID)
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

// Mock commands acknowledge deletion without a database snapshot. Keep all
// unrelated statistics while removing only the acknowledged rows.
func (m *Model) removeAcknowledgedSessions(ids []session.Ref) {
	removed := make(map[session.Ref]bool, len(ids))
	for _, id := range ids {
		removed[id] = true
	}
	remaining := make([]session.Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		if removed[session.Ref] {
			m.totalSessions--
		} else {
			remaining = append(remaining, session)
		}
	}
	m.sessions = remaining
	m.selected = make(map[session.Ref]bool)
	m.deletePhase = deleteNone
	m.rebuildRows()
	if m.table.Cursor() >= len(m.sessions) {
		m.table.SetCursor(max(len(m.sessions)-1, 0))
	}
}

func (m *Model) applyData(msg reloadedMsg) {
	snap := msg.snapshot
	m.sessions = snap.Sessions
	m.totalSessions = snap.TotalCount
	m.dbUsed, m.dbTotal = 0, 0
	m.metricsKnown = snap.Metrics != nil
	if snap.Metrics != nil {
		m.dbUsed, m.dbTotal = snap.Metrics.Used, snap.Metrics.Total
	}
	m.orphanCount, m.orphanBytes = 0, 0
	if snap.Orphans != nil {
		m.orphanCount, m.orphanBytes = snap.Orphans.Count, snap.Orphans.Bytes
	}
	m.periodCost = 0
	if snap.PeriodCost != nil {
		m.periodCost = snap.PeriodCost.Amount
	}
	m.capabilities = snap.Capabilities
	m.updateKeys()
	m.diagnostics = snap.Diagnostics
	m.selected = make(map[session.Ref]bool)
	m.deletePhase = deleteNone
	m.rebuildRows()
	if len(m.sessions) == 0 {
		if m.history.backend != nil {
			m.history.active = true
		}
		m.table.SetCursor(0)
	} else if m.table.Cursor() >= len(m.sessions) {
		m.table.SetCursor(len(m.sessions) - 1)
	}
}

func (m *Model) updateKeys() {
	m.keys.History.SetEnabled(m.history.backend != nil)
	m.keys.Enter.SetEnabled(m.capabilities.Resume)
	m.keys.Delete.SetEnabled(m.capabilities.Delete)
	m.keys.Vacuum.SetEnabled(m.capabilities.Vacuum.Available)
	m.keys.Prune.SetEnabled(m.capabilities.Orphans.Available)
}

func (m Model) View() tea.View {
	if m.history.backend != nil && m.history.active {
		return m.historyView()
	}
	titleBar := m.renderTitleBar()
	var statusText string
	switch {
	case m.reloading:
		statusText = styleWarning.Render("reloading...")
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
			statusLine = ui.StatusBarStyle().Render(
				lipgloss.PlaceHorizontal(innerWidth, lipgloss.Right, statusText))
		} else {
			statusLine = ui.StatusBarStyle().Render("")
		}
		footer = lipgloss.JoinVertical(lipgloss.Left,
			statusLine,
			ui.StatusBarStyle().Render(helpRendered),
		)
	} else {
		var footerContent string
		if statusText != "" {
			rightAligned := lipgloss.PlaceHorizontal(innerWidth-lipgloss.Width(helpRendered), lipgloss.Right, statusText)
			footerContent = helpRendered + rightAligned
		} else {
			footerContent = helpRendered
		}
		footer = ui.StatusBarStyle().Render(footerContent)
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

func (m *Model) rebuildRows() {
	_, rows := buildTable(m.sessions, m.selected, m.width-borderWidth, m.homeDir)
	m.table.SetRows(rows)
}

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
	right := sessionLabel
	if m.metricsKnown {
		right += "  ·  " + sizeLabel
	}
	if m.costPeriod != "" {
		right += "  ·  " + string(m.costPeriod) + ": " + fmt.Sprintf("$%.2f", m.periodCost)
	}

	styledLeft := lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true).Render(left)
	styledRight := lipgloss.NewStyle().Foreground(ui.ColorText).Render(right)

	innerWidth := m.width - 2
	rightAligned := lipgloss.PlaceHorizontal(innerWidth-lipgloss.Width(left), lipgloss.Right, styledRight)
	return ui.TitleBarStyle().Render(styledLeft + rightAligned)
}

// A failed guarded action still reloads authoritatively. Keep marks/cursor only
// for refs which survived that reload, so the user can inspect and retry safely.
func (m *Model) applyDeleteData(msg reloadedMsg, failed bool) {
	old := m.selected
	var cursor session.Ref
	idx := m.table.Cursor()
	if idx >= 0 && idx < len(m.sessions) {
		cursor = m.sessions[idx].Ref
	}
	m.applyData(msg)
	if !failed {
		return
	}
	for i, row := range m.sessions {
		if old[row.Ref] {
			m.selected[row.Ref] = true
		}
		if row.Ref == cursor {
			m.table.SetCursor(i)
		}
	}
	m.rebuildRows()
}
