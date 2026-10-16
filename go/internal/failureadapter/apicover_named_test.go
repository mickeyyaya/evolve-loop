package failureadapter

import "testing"

func TestDecision_VerdictNoneOnProceed(t *testing.T) {
	var proceed Decision = Decide(nil, Options{Now: fixedTime()})
	if proceed.Action != ActionProceed {
		t.Fatalf("Action=%q want PROCEED", proceed.Action)
	}
	if proceed.VerdictForBlock != VerdictNone {
		t.Errorf("PROCEED must carry VerdictNone; got %q", proceed.VerdictForBlock)
	}

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
