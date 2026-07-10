package core

// recurrence_generic_escalation_c662_test.go — cycle-662 RED contract for
// chronicle-s1-recurrence-index gap G3, consumer side. escalateRetroReason
// (decision_branch.go) upgrades a deterministic "proceed:" retro reason to an
// "adapt:"-with-escalation reason once a pattern recurs (count>=2). Post-backfill
// the corpus is dominated by generic noise (operator-reset x96, loop-fatal x62);
// escalation MUST consume NON-generic patterns only, or every FAIL cycle would
// be force-escalated by the noise floor.
//
// Builder contract: escalateRetroReason must return the reason unchanged when the
// pattern is generic (led.IsGenericPattern(pattern) == true), even at count>=2.
//
// Internal test (package core) — escalateRetroReason and recurrence.Entry fields
// are exercised directly. RED today: recurrence.Entry has no Generic field, so
// package core fails to compile.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// TestC662_EscalateRetroReasonIgnoresGenericPatterns — AC4b. A generic pattern at
// count>=2 must NOT escalate (bare "proceed" stands); a non-generic pattern at
// the same count MUST escalate. Both directions in one table proves the fix
// strips only noise and is not a blanket disable.
func TestC662_EscalateRetroReasonIgnoresGenericPatterns(t *testing.T) {
	const reason = "proceed: no failures requiring adaptation"

	genericLed := recurrence.NewLedger()
	genericLed.Entries["operator-reset"] = &recurrence.Entry{
		Pattern: "operator-reset", Cycles: []int{1, 2}, Count: 2, Generic: true,
	}
	if got := escalateRetroReason(reason, "operator-reset", genericLed); got != reason {
		t.Errorf("generic pattern at count=2 was escalated:\n got  %q\n want %q (unchanged — generic noise must not escalate)", got, reason)
	}

	realLed := recurrence.NewLedger()
	realLed.Entries["builder-out-of-lane-ships-red"] = &recurrence.Entry{
		Pattern: "builder-out-of-lane-ships-red", Cycles: []int{1, 2}, Count: 2, Generic: false,
	}
	got := escalateRetroReason(reason, "builder-out-of-lane-ships-red", realLed)
	if !strings.HasPrefix(got, "adapt:") {
		t.Errorf("non-generic pattern at count=2 did not escalate:\n got %q\n want an 'adapt:' escalation", got)
	}
}
