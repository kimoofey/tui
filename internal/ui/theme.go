package ui

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

var (
	// Polar Night — dark backgrounds
	nord0 = lipgloss.Color("#2E3440")
	nord1 = lipgloss.Color("#3B4252")
	nord2 = lipgloss.Color("#434C5E")
	nord3 = lipgloss.Color("#4C566A")
	// Snow Storm — text
	nord4 = lipgloss.Color("#D8DEE9")
	nord5 = lipgloss.Color("#E5E9F0")
	nord6 = lipgloss.Color("#ECEFF4")
	// Frost — blue/cyan accents
	nord7  = lipgloss.Color("#8FBCBB")
	nord8  = lipgloss.Color("#88C0D0")
	nord9  = lipgloss.Color("#81A1C1")
	nord10 = lipgloss.Color("#5E81AC")
	// Aurora — semantic colors
	nord11 = lipgloss.Color("#BF616A")
	nord12 = lipgloss.Color("#D08770")
	nord13 = lipgloss.Color("#EBCB8B")
	nord14 = lipgloss.Color("#A3BE8C")
	nord15 = lipgloss.Color("#B48EAD")
)

// Mapped per Nord design guidelines: https://www.nordtheme.com/docs/colors-and-palettes
// Exported so individual apps can override them before constructing their model.

var (
	ColorBg       = compat.AdaptiveColor{Dark: nord0, Light: nord6}
	ColorElevated = compat.AdaptiveColor{Dark: nord1, Light: nord4}
	ColorSelected = compat.AdaptiveColor{Dark: nord2, Light: nord5}
	ColorMuted    = compat.AdaptiveColor{Dark: nord3, Light: nord3}
	ColorText     = compat.AdaptiveColor{Dark: nord4, Light: nord0}
	ColorAccent   = compat.AdaptiveColor{Dark: nord8, Light: nord8}
	ColorError    = compat.AdaptiveColor{Dark: nord11, Light: nord11}
	ColorWarning  = compat.AdaptiveColor{Dark: nord13, Light: nord13}
	ColorSuccess  = compat.AdaptiveColor{Dark: nord14, Light: nord14}
)

// suppress unused-var warnings for palette entries not used in color roles
var _ = nord7
var _ = nord9
var _ = nord10
var _ = nord12
var _ = nord15
