// Package textcap is the ONE home of the two rune-cap text rules the phase
// advisor's prompt (unit 04 of ADR-0103) and the carryover-todo lifecycle
// (unit 03) both apply: TruncateRunes trims and marks with " …[truncated]",
// CapRunes marks with the bare ellipsis rune. Stdlib only; the two consumers
// project it so neither imports the other for a string rule. Design:
// docs/architecture/decomposition/04-advisor.md.
package textcap

import "strings"

// TruncateRunes trims surrounding whitespace and caps s at max runes, marking
// truncation with " …[truncated]" — the advisor goal/card-hint rule and the
// remediation title's rule.
func TruncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + " …[truncated]"
}

// CapRunes truncates s to maxRunes with a trailing ellipsis rune — the
// carryover action rule, never trimming.
func CapRunes(s string, maxRunes int) string {
	if r := []rune(s); len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return s
}
