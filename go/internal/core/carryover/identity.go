package carryover

import (
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// cycleTokenRE folds the cycle number out of a todo's action so the same
// failure class re-minted in a later cycle fingerprints identically (the
// 2026-08-10 flood: 124 of 254 entries were per-cycle re-mints).
var cycleTokenRE = regexp.MustCompile(`(?i)\bcycle[ -]\d+`)

// fingerprint is the identity of a todo's action across cycles: the cycle
// token folded, lower-cased, whitespace collapsed.
func fingerprint(action string) string {
	s := cycleTokenRE.ReplaceAllString(action, "cycle-N")
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// fingerprintIndex returns the FIRST twin of action in todos, or -1.
func fingerprintIndex(todos []cyclestate.CarryoverTodo, action string) int {
	fp := fingerprint(action)
	for i, t := range todos {
		if fingerprint(t.Action) == fp {
			return i
		}
	}
	return -1
}

// HasID reports whether a todo with id is present.
func HasID(todos []cyclestate.CarryoverTodo, id string) bool {
	for _, t := range todos {
		if t.ID == id {
			return true
		}
	}
	return false
}
