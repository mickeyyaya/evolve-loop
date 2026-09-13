package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
)

// ADR-0103 unit 03b: truncateGoal is a CONSUMER of carryover.TruncateRunes —
// the advisor's goal cap renders the unit's marker, so the core facade cannot
// silently re-implement the rule.
func TestTruncateGoal_ProjectsTheCarryoverCap(t *testing.T) {
	long := strings.Repeat("g", maxGoalTextChars+7)
	if got, want := truncateGoal("  "+long+"  "), carryover.TruncateRunes(long, maxGoalTextChars); got != want {
		t.Fatalf("truncateGoal must be the carryover cap at maxGoalTextChars:\n got %q\nwant %q", got[len(got)-20:], want[len(want)-20:])
	}
}
