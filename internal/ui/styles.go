package ui

import (
	"os"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// PrePaint is written to stdout before bubbletea starts to prevent the white
// flash that occurs while the alt-screen buffer is still empty.
//
// Sequence:
//
//	\033[?1049h          — enter alt screen
//	\033[48;5;236m       — SGR: set cell bg to a dark 256-color approximation of nord0
//	\033[2J              — erase display (all cells filled with dark bg)
//
// Use 256-color SGR because Apple Terminal does not support 24-bit background
// colors. OSC 11 is omitted to avoid corrupting Bubble Tea's dark/light
// detection. v2 writes \033[?1049l on quit.
const PrePaint = "\033[?1049h\033[48;5;236m\033[2J\033[0m"

// ProgramOptions limits Apple Terminal to its supported palette even when the
// surrounding environment advertises truecolor. Preserve lower-color settings.
func ProgramOptions() []tea.ProgramOption {
	profile := colorprofile.Detect(os.Stdout, os.Environ())
	if appleTerminalNeedsColorLimit(os.Getenv("TERM_PROGRAM"), profile) {
		return []tea.ProgramOption{tea.WithColorProfile(colorprofile.ANSI256)}
	}
	return nil
}

func appleTerminalNeedsColorLimit(program string, profile colorprofile.Profile) bool {
	return program == "Apple_Terminal" && profile > colorprofile.ANSI256
}

// HelpStyles returns Nord-themed styles for the bubbles/help component.
// Both apps use identical styles so this is the single source of truth.
func HelpStyles() help.Styles {
	return help.Styles{
		ShortKey:       lipgloss.NewStyle().Foreground(ColorAccent),
		ShortDesc:      lipgloss.NewStyle().Foreground(ColorText),
		ShortSeparator: lipgloss.NewStyle().Foreground(ColorSubtleText),
		Ellipsis:       lipgloss.NewStyle().Foreground(ColorSubtleText),
		FullKey:        lipgloss.NewStyle().Foreground(ColorAccent),
		FullDesc:       lipgloss.NewStyle().Foreground(ColorText),
		FullSeparator:  lipgloss.NewStyle().Foreground(ColorSubtleText),
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
