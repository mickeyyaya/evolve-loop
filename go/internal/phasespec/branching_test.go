package phasespec

// branching_test.go — PA-BIG S2 (ADR-0058): wire-contract lock for the
// branching_strategy vocabulary. Pins the strategy const values and the JSON
// key so a registry entry's branching_strategy survives load into the catalog
// (the orchestrator's successorStrategy reads PhaseSpec.BranchingStrategy).

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBranchingStrategy_ConstsAndJSONTag asserts the strategy wire values are
// stable and distinct, and that PhaseSpec.BranchingStrategy round-trips under
// its documented `branching_strategy` JSON key.
func TestBranchingStrategy_ConstsAndJSONTag(t *testing.T) {
	if BranchingVerdict != "verdict" || BranchingHistory != "history" {
		t.Fatalf("branching strategy wire values drifted: verdict=%q history=%q", BranchingVerdict, BranchingHistory)
	}
	if BranchingHistory == BranchingVerdict {
		t.Fatal("branching strategies must be distinct")
	}

	raw, err := json.Marshal(PhaseSpec{Name: "retrospective", BranchingStrategy: BranchingHistory})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"branching_strategy":"history"`) {
		t.Errorf("PhaseSpec must serialize BranchingStrategy under branching_strategy:\n%s", raw)
	}

	var rt PhaseSpec
	if err := json.Unmarshal(raw, &rt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rt.BranchingStrategy != BranchingHistory {
		t.Errorf("branching_strategy round-trip = %q, want %q", rt.BranchingStrategy, BranchingHistory)
	}

	// omitempty: an unset strategy must not appear in the wire form (keeps
	// minimal phase.json minimal and the default implicit).
	bare, _ := json.Marshal(PhaseSpec{Name: "scout"})
	if strings.Contains(string(bare), "branching_strategy") {
		t.Errorf("empty BranchingStrategy must be omitted from JSON:\n%s", bare)
	}
}
