package dossier

import (
	"encoding/json"
	"testing"
)

// TestRender verifies RenderJSON produces valid round-trip JSON and
// RenderMarkdown produces non-empty output. RED: RenderJSON/RenderMarkdown
// don't exist yet.
func TestRender(t *testing.T) {
	d := &Dossier{
		Cycle:        3,
		Goal:         "render-round-trip goal",
		FinalVerdict: VerdictPass,
		Phases:       []PhaseRecord{{Name: "build", Verdict: VerdictPass}},
	}
	raw, err := RenderJSON(d)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var round Dossier
	if err := json.Unmarshal(raw, &round); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	if round.Cycle != d.Cycle {
		t.Errorf("JSON round-trip Cycle: got %d, want %d", round.Cycle, d.Cycle)
	}
	if round.Goal != d.Goal {
		t.Errorf("JSON round-trip Goal: got %q, want %q", round.Goal, d.Goal)
	}
	md, err := RenderMarkdown(d)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if len(md) == 0 {
		t.Error("RenderMarkdown: empty output — must produce non-empty markdown")
	}
}
