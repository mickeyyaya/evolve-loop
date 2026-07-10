package recurrence

// generic.go — cycle-662 gap G3 (GENERIC CLASSIFICATION de-noise). A per-pattern
// Generic flag separates classification-vocabulary noise (operator-reset x96,
// loop-fatal x62 — 59% of the corpus) from specific-defect signal so escalation
// and the CLI report act on actionable patterns only. Backfill sets Entry.Generic
// via IsGeneric; consumers read the stored flag via IsGenericPattern.

import "strings"

// genericPatternDenylist is the classification-vocabulary noise set: patterns
// that are error-category labels (not specific defects) and therefore dominate
// raw recurrence counts without carrying an actionable signal. Kept small and
// explicit — the pattern==errorCategory echo rule in IsGeneric catches the rest.
var genericPatternDenylist = map[string]bool{
	"operator-reset":           true,
	"loop-fatal":               true,
	"cycle-mid-execution-fail": true,
	"cycle-fatal":              true,
	"unknown":                  true,
	"unknown-classification":   true,
}

// IsGeneric reports whether pattern is classification-vocabulary noise rather
// than a specific defect. True when the pattern is on the denylist, is empty, or
// verbatim echoes the lesson's failureContext.errorCategory (the deterministic-
// floor shape, where pattern==errorCategory). A specific LLM defect whose pattern
// differs from its errorCategory is NOT generic — exactly the signal to keep.
func IsGeneric(pattern, errorCategory string) bool {
	p := strings.TrimSpace(pattern)
	if p == "" {
		return true
	}
	if genericPatternDenylist[p] {
		return true
	}
	if ec := strings.TrimSpace(errorCategory); ec != "" && p == ec {
		return true
	}
	return false
}

// IsGenericPattern reports the stored Generic flag for pattern (false for an
// unseen pattern or a nil ledger). Consumers gate escalation / CLI rendering on
// this so generic noise never escalates or clutters the report.
func (l *Ledger) IsGenericPattern(pattern string) bool {
	if l == nil {
		return false
	}
	if e, ok := l.Entries[pattern]; ok {
		return e.Generic
	}
	return false
}
