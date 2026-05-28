package ui

import "os"

// ResolveTerminal returns the terminal identifier to use for launching external
// processes. cfgVal is an optional user override (from config); if empty,
// $TERM_PROGRAM is consulted for auto-detection.
//
// Return values used by callers:
//   - "terminal"  — macOS Terminal.app (AppleScript)
//   - "ghostty"   — Ghostty via `open -na Ghostty`
//   - "iterm2"    — iTerm2 (AppleScript)
//   - anything else — treated as a custom launch prefix, e.g. "kitty --"
func ResolveTerminal(cfgVal string) string {
	if cfgVal != "" {
		return cfgVal
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "Apple_Terminal":
		return "terminal"
	case "ghostty":
		return "ghostty"
	case "iTerm.app":
		return "iterm2"
	default:
		return "terminal"
	}
}
