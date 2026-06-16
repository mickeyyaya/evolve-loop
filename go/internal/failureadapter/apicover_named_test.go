package failureadapter

import "testing"

// TestDecision_VerdictNoneOnProceed names both the Decision type and the
// VerdictNone const, pinning the "no block ⇒ no verdict" invariant: a PROCEED
// decision leaves VerdictForBlock at VerdictNone (the empty Verdict), while a
// strict-mode block populates a concrete verdict. This couples the named zero
// value to the real Decide code path rather than asserting a bare "".
func TestDecision_VerdictNoneOnProceed(t *testing.T) {
	// No failures → PROCEED, and the block verdict must be the VerdictNone zero.
	var proceed Decision = Decide(nil, Options{Now: fixedTime()})
	if proceed.Action != ActionProceed {
		t.Fatalf("Action=%q want PROCEED", proceed.Action)
	}
	if proceed.VerdictForBlock != VerdictNone {
		t.Errorf("PROCEED must carry VerdictNone; got %q", proceed.VerdictForBlock)
	}

	// A strict-mode intent-rejected block must set a concrete, non-None verdict.
	blocked := Decide(
		[]Entry{{Cycle: 1, Classification: IntentRejected}},
		Options{Strict: true, Now: fixedTime()},
	)
	if blocked.Action != ActionBlockCode {
		t.Fatalf("Action=%q want BLOCK-CODE", blocked.Action)
	}
	if blocked.VerdictForBlock == VerdictNone {
		t.Errorf("a block decision must set a non-None verdict; got VerdictNone")
	}
	if blocked.VerdictForBlock != VerdictScopeRejected {
		t.Errorf("intent-rejected block verdict=%q want %q", blocked.VerdictForBlock, VerdictScopeRejected)
	}
}
