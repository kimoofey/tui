package prq

import "charm.land/bubbles/v2/key"

// KeyMap defines all keybindings for the TUI.
// It implements help.KeyMap so the bubbles/help component can render it.
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	NextTab  key.Binding
	Enter    key.Binding
	Refresh  key.Binding
	OpenCode key.Binding
	Esc      key.Binding
	Help     key.Binding
	Quit     key.Binding
}

// DefaultKeyMap is the default set of keybindings.
var DefaultKeyMap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup", "b"),
		key.WithHelp("pgup/b", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown", "f"),
		key.WithHelp("pgdn/f", "page down"),
	),
	NextTab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch tabs"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "browser"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	OpenCode: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "opencode"),
	),
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "more keys"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// ShortHelp returns the bindings shown in the collapsed (one-line) help bar.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.OpenCode, k.Refresh, k.Help, k.Quit}
}

// FullHelp returns all bindings grouped into columns for the expanded help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown},
		{k.NextTab, k.Enter, k.Refresh},
		{k.OpenCode, k.Esc, k.Help, k.Quit},
	}
}
