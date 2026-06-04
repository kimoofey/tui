package prq

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kimoofey/tui/internal/ui"
)

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

	innerWidth := m.width - 2
	rightAligned := lipgloss.PlaceHorizontal(innerWidth-lipgloss.Width(left), lipgloss.Right, styledRight)
	return ui.TitleBarStyle().Render(styledLeft + rightAligned)
}

func (m Model) renderStatus() string {
	normal := lipgloss.NewStyle().Foreground(ui.ColorText)
	switch {
	case m.loading:
		return normal.Render("Fetching PRs…")
	case m.statusMsg != "":
		return m.statusMsg
	default:
		return normal.Render(fmt.Sprintf("updated %s", m.lastFetched.Format("15:04")))
	}
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
	tableH := max(m.tableHeight(), 1)

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
