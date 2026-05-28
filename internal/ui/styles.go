package ui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"
)

// PrePaint is written to stdout before bubbletea starts to prevent the white
// flash that occurs while the alt-screen buffer is still empty.
//
// Sequence:
//
//	\033[?1049h          — enter alt screen
//	\033[48;2;46;52;64m  — SGR: set cell bg to nord0 (#2E3440, rgb 46,52,64)
//	\033[2J              — erase display (all cells filled with dark bg)
//
// OSC 11 is deliberately omitted to avoid corrupting bubbletea's dark/light
// detection. v2 writes \033[?1049l on quit.
const PrePaint = "\033[?1049h\033[48;2;46;52;64m\033[2J\033[0m"

// HelpStyles returns Nord-themed styles for the bubbles/help component.
// Both apps use identical styles so this is the single source of truth.
func HelpStyles() help.Styles {
	return help.Styles{
		ShortKey:       lipgloss.NewStyle().Foreground(ColorAccent),
		ShortDesc:      lipgloss.NewStyle().Foreground(ColorText),
		ShortSeparator: lipgloss.NewStyle().Foreground(ColorMuted),
		Ellipsis:       lipgloss.NewStyle().Foreground(ColorMuted),
		FullKey:        lipgloss.NewStyle().Foreground(ColorAccent),
		FullDesc:       lipgloss.NewStyle().Foreground(ColorText),
		FullSeparator:  lipgloss.NewStyle().Foreground(ColorMuted),
	}
}

// TitleBarStyle returns the base style for the top title bar (transparent bg,
// 1-cell horizontal padding).
func TitleBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().Padding(0, 1)
}

// StatusBarStyle returns the style for the footer status/help bar (transparent
// bg, 1-cell horizontal padding).
func StatusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().Padding(0, 1)
}
