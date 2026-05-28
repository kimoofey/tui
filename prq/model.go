package prq

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	ui "github.com/kimoofey/tui/internal/ui"
)

// fetchDoneMsg is sent on the Bubble Tea bus when FetchAll completes.
type fetchDoneMsg FetchResult

// fetchCmd wraps FetchAll in a tea.Cmd so it runs off the main goroutine.
func fetchCmd(cfg Config) tea.Cmd {
	return func() tea.Msg {
		result := FetchAll(cfg)
		return fetchDoneMsg(result)
	}
}

// Model is the Bubble Tea application model.
type Model struct {
	table   table.Model
	spinner spinner.Model
	help    help.Model
	keys    KeyMap

	reviewPRs  []PullRequest
	myPRs      []PullRequest
	currentTab int

	cfg         Config
	lastFetched time.Time

	loading   bool
	fetchErr  error
	statusMsg string // non-empty overrides the default status line

	width  int
	height int
}

// NewModel constructs the initial Model with an empty table and running spinner.
func NewModel(cfg Config) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.ColorAccent)

	h := help.New()
	h.ShowAll = false
	h.Styles = ui.HelpStyles()

	t := table.New(
		table.WithColumns(makeColumns(80)),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(20),
		table.WithWidth(80),
		table.WithStyles(makeTableStyles()),
	)

	keys := DefaultKeyMap
	if !IsOpencodeAvailable() {
		keys.OpenCode.SetEnabled(false)
	}

	return Model{
		table:   t,
		spinner: s,
		help:    h,
		keys:    keys,
		cfg:     cfg,
		loading: true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		fetchCmd(m.cfg),
		tea.RequestBackgroundColor,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width - 2)
		m = m.resized()

	case tea.MouseWheelMsg:
		if !m.loading {
			switch msg.Button {
			case tea.MouseWheelUp:
				m.table.MoveUp(3)
			case tea.MouseWheelDown:
				m.table.MoveDown(3)
			}
		}

	case fetchDoneMsg:
		m.loading = false
		m.lastFetched = time.Now()
		m.reviewPRs = msg.ReviewPRs
		m.myPRs = msg.MyPRs
		if msg.Err != nil {
			m.fetchErr = msg.Err
			m.statusMsg = styleError.Render("✗ " + msg.Err.Error())
		} else {
			m.fetchErr = nil
			m.statusMsg = ""
		}
		m = m.resized()
		m.table.GotoTop()

	case openLaunchErrMsg:
		m.statusMsg = styleError.Render("✗ open: " + msg.Err.Error())

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Refresh) && !m.loading:
			m.loading = true
			m.statusMsg = ""
			return m, tea.Batch(m.spinner.Tick, fetchCmd(m.cfg))

		case key.Matches(msg, m.keys.NextTab):
			m.currentTab = (m.currentTab + 1) % 2
			m = m.resized()
			m.table.GotoTop()

		case key.Matches(msg, m.keys.Enter):
			prs := m.currentPRs()
			if idx := m.table.Cursor(); idx >= 0 && idx < len(prs) {
				return m, OpenInBrowser(prs[idx].URL)
			}

		case key.Matches(msg, m.keys.OpenCode):
			prs := m.currentPRs()
			if idx := m.table.Cursor(); idx >= 0 && idx < len(prs) {
				return m, OpenInTerminal(prs[idx].URL, m.cfg)
			}

		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			m = m.resized()

		case key.Matches(msg, m.keys.Esc):
			if m.help.ShowAll {
				m.help.ShowAll = false
				m = m.resized()
			}

		default:
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("\n  " + m.spinner.View() + " Starting…\n")
		v.AltScreen = true
		return v
	}

	var tableContent string
	if !m.loading && len(m.currentPRs()) == 0 && m.fetchErr == nil {
		tableContent = m.renderEmptyState()
	} else {
		tableContent = m.table.View()
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		m.renderTitleBar(),
		m.renderTabBar(),
		styleBase.Render(tableContent),
		m.renderFooter(),
	)

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) renderTabBar() string {
	tabs := []string{"Review Queue", "My PRs"}
	var parts []string
	for i, label := range tabs {
		if i == m.currentTab {
			parts = append(parts, activeTabStyle.Render(label))
		} else {
			parts = append(parts, inactiveTabStyle.Render(label))
		}
		if i < len(tabs)-1 {
			parts = append(parts, tabSepStyle.Render("│"))
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, parts...)
	labels := tabRowStyle.Width(m.width).Render(row)
	border := tabBorderLine(m.width)
	return labels + "\n" + border
}

func (m Model) renderTitleBar() string {
	left := "prq"
	var right string
	if !m.loading {
		prs := m.currentPRs()
		switch {
		case len(prs) == 0:
			right = "no PRs"
		case len(prs) == 1:
			right = "1 PR"
		default:
			right = fmt.Sprintf("%d PRs", len(prs))
		}
	}

	styledLeft := lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true).Render(left)
	styledRight := lipgloss.NewStyle().Foreground(ui.ColorText).Render(right)

	innerWidth := m.width - 2 // -2 for Padding(0,1) on each side
	rightAligned := lipgloss.PlaceHorizontal(innerWidth-lipgloss.Width(left), lipgloss.Right, styledRight)
	return ui.TitleBarStyle().Render(styledLeft + rightAligned)
}

func (m Model) renderStatus() string {
	prs := m.currentPRs()
	normal := lipgloss.NewStyle().Foreground(ui.ColorText)
	switch {
	case m.loading:
		return normal.Render("Fetching PRs…")
	case m.statusMsg != "":
		return m.statusMsg
	case m.lastFetched.IsZero():
		return normal.Render(fmt.Sprintf("%d PR(s) %s", len(prs), m.tabStatusLabel()))
	default:
		return normal.Render(fmt.Sprintf("%d PR(s) %s  ·  updated %s",
			len(prs), m.tabStatusLabel(), m.lastFetched.Format("15:04")))
	}
}

func (m Model) tabStatusLabel() string {
	if m.currentTab == 0 {
		return "ready for review"
	}
	return "open"
}

func (m Model) renderFooter() string {
	statusText := m.renderStatus()
	helpRendered := m.help.View(m.keys)
	innerWidth := m.width - 2

	if m.help.ShowAll {
		var statusLine string
		if statusText != "" {
			statusLine = ui.StatusBarStyle().Render(
				lipgloss.PlaceHorizontal(innerWidth, lipgloss.Right, statusText))
		} else {
			statusLine = ui.StatusBarStyle().Render("")
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			statusLine,
			ui.StatusBarStyle().Render(helpRendered),
		)
	}

	if statusText != "" {
		rightAligned := lipgloss.PlaceHorizontal(innerWidth-lipgloss.Width(helpRendered), lipgloss.Right, statusText)
		return ui.StatusBarStyle().Render(helpRendered + rightAligned)
	}
	return ui.StatusBarStyle().Render(helpRendered)
}

func (m Model) renderEmptyState() string {
	tableH := m.tableHeight()
	if tableH < 1 {
		tableH = 1
	}

	var emptyMsg string
	if m.currentTab == 0 {
		emptyMsg = "✓  All caught up! No PRs need your attention right now.  r to refresh."
	} else {
		emptyMsg = "No open PRs found.  r to refresh."
	}

	msg := emptyStyle().Render(emptyMsg)
	topPad := (tableH - 1) / 2
	bottomPad := tableH - 1 - topPad

	centered := lipgloss.PlaceHorizontal(m.width-borderWidth, lipgloss.Center, msg)
	return strings.Repeat("\n", topPad) + centered + strings.Repeat("\n", bottomPad)
}

func (m Model) helpLines() int {
	if !m.help.ShowAll {
		return 1
	}
	max := 0
	for _, col := range m.keys.FullHelp() {
		if len(col) > max {
			max = len(col)
		}
	}
	return max
}

func (m Model) footerHeight() int {
	if m.help.ShowAll {
		return 1 + m.helpLines()
	}
	return 1
}

// tableHeight returns the number of lines the table occupies in the layout.
// Layout: title(1) + tabBar(2) + border-top(1) + table(h) + border-bottom(1) + footer
func (m Model) tableHeight() int {
	h := m.height - 5 - m.footerHeight()
	if h < 3 {
		h = 3
	}
	return h
}

func (m Model) resized() Model {
	if m.width == 0 || m.height == 0 {
		return m
	}
	m.table.SetHeight(m.tableHeight())
	m.table.SetWidth(m.width - borderWidth)
	m.table.SetRows([]table.Row{})
	m.table.SetColumns(m.currentColumns())
	m.table.SetRows(m.currentRows())
	m.table.SetStyles(makeTableStyles())
	return m
}

func (m Model) currentPRs() []PullRequest {
	if m.currentTab == 0 {
		return m.reviewPRs
	}
	return m.myPRs
}

func (m Model) currentColumns() []table.Column {
	if m.currentTab == 0 {
		return makeColumns(m.width - borderWidth)
	}
	return makeMyColumns(m.width - borderWidth)
}

func (m Model) currentRows() []table.Row {
	if m.currentTab == 0 {
		return PRsToRows(m.reviewPRs, m.cfg.MinApprovals)
	}
	return MyPRsToRows(m.myPRs)
}
