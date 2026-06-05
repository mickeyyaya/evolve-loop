package channel

import "testing"

func TestStallPolicy_AsksOnStall(t *testing.T) {
	pol := StallPolicy{Question: "Summarize progress + blockers in 3 bullets."}
	act := pol.OnEvent(map[string]any{"kind": "stall"})
	if act == nil || act.Question != "Summarize progress + blockers in 3 bullets." {
		t.Fatalf("expected ask action, got %+v", act)
	}
	if pol.OnEvent(map[string]any{"kind": "assistant_text"}) != nil {
		t.Fatalf("non-stall must not ask")
	}
}
