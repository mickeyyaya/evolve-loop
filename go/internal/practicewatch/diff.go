package practicewatch

import (
	"strings"
	"unicode/utf8"
)

const (
	excerptLimit     = 64 * 1024
	addedLinesLimit  = 4 * 1024
	truncationNotice = "\n[... added lines truncated ...]"
	noAddedLines     = "(no added line in the stored excerpt: the change lies outside it or only removed text)"
)

func addedLines(before, after string, limit int) string {
	seen := map[string]int{}
	for _, line := range strings.Split(before, "\n") {
		seen[line]++
	}
	var added []string
	for _, line := range strings.Split(after, "\n") {
		if seen[line] > 0 {
			seen[line]--
			continue
		}
		added = append(added, line)
	}
	body := strings.Join(added, "\n")
	if strings.TrimSpace(body) == "" {
		return noAddedLines
	}
	if capped := capText(body, limit); capped != body {
		return capped + truncationNotice
	}
	return body
}

func capText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
