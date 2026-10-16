// Package textcap holds the two rune-cap rules for text that is shown or carried.
// It is stdlib only, so each consumer can project it without importing the other.
package textcap

import "strings"

const (
	truncatedMarker = " …[truncated]"
	ellipsisMarker  = "…"
)

func TruncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + truncatedMarker
}

func CapRunes(s string, maxRunes int) string {
	if r := []rune(s); len(r) > maxRunes {
		return string(r[:maxRunes]) + ellipsisMarker
	}
	return s
}
