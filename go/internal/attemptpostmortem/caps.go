package attemptpostmortem

import "strings"

const ellipsis = "…"

const maxFieldRunes = 120

func stripControls(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

func capHead(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit-1]) + ellipsis
}

func capTail(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return ellipsis + string(runes[len(runes)-limit+1:])
}
