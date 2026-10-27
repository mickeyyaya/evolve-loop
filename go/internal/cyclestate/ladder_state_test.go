package cyclestate

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCycleState_LadderStateRoundTripsAndIsOmittedWhenAbsent(t *testing.T) {
	bare, err := json.Marshal(CycleState{CycleID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bare), "ladder") {
		t.Fatalf("a cycle without a ladder decision writes %s", bare)
	}
	in := CycleState{CycleID: 7, Ladder: &LadderState{
		Rung: "R2", Tier: "deep", Effort: "high", Fingerprints: []string{"codes:a"}, History: []string{"R0 capability codes:a"}, RepairRound: 1, AuditFix: "would",
	}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out CycleState
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Ladder == nil || out.Ladder.Rung != "R2" || out.Ladder.Tier != "deep" || out.Ladder.Effort != "high" ||
		out.Ladder.Fingerprints[0] != "codes:a" || out.Ladder.History[0] != "R0 capability codes:a" || out.Ladder.RepairRound != 1 || out.Ladder.AuditFix != "would" {
		t.Fatalf("round trip = %+v from %s", out.Ladder, raw)
	}
}
