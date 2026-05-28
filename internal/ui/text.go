package ui

// Truncate shortens s to max runes, appending "…" if it was cut.
// Returns "…" when max <= 1 so there is always at least one visible character.
func Truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}
