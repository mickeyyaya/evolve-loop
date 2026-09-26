package phasespec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBranchingStrategy_ConstsAndJSONTag(t *testing.T) {
	if BranchingVerdict != "verdict" || BranchingHistory != "history" || BranchingSignal != "signal" {
		t.Fatalf("branching strategy wire values drifted: verdict=%q history=%q signal=%q",
			BranchingVerdict, BranchingHistory, BranchingSignal)
	}
	distinct := map[string]bool{BranchingVerdict: true, BranchingHistory: true, BranchingSignal: true}
	if len(distinct) != 3 {
		t.Fatalf("branching strategies must be distinct: verdict=%q history=%q signal=%q",
			BranchingVerdict, BranchingHistory, BranchingSignal)
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

	bare, _ := json.Marshal(PhaseSpec{Name: "scout"})
	if strings.Contains(string(bare), "branching_strategy") {
		t.Errorf("empty BranchingStrategy must be omitted from JSON:\n%s", bare)
	}
}
