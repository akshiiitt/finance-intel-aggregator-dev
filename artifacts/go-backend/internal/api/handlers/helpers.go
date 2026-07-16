package handlers

import (
	"strings"
)

// joinComma joins a slice with ", " separator.
// Used by partial-update handlers to build SET clauses.
func joinComma(parts []string) string {
	return strings.Join(parts, ", ")
}

// truncateStr clips s to maxLen bytes, appending "…" if trimmed.
// Named truncateStr to avoid conflict with the ai package's truncate.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}
