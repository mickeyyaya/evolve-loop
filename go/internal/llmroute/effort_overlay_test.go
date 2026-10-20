package llmroute

import (
	"slices"
	"testing"
)

func TestApplySoftOverlay_AnEffortOverlaySetsOnlyThePlanEffort(t *testing.T) {
	in := Plan{Candidates: []string{"claude-tmux"}, Model: "balanced", Tiers: []string{"balanced"}}

	out := ApplySoftOverlay(in, Overlay{Effort: "high"}, nil)
	if out.Effort != "high" || out.Model != "balanced" || !slices.Equal(out.Tiers, in.Tiers) || !slices.Equal(out.Candidates, in.Candidates) {
		t.Fatalf("plan %+v, want only the effort set to high", out)
	}
	if kept := ApplySoftOverlay(out, Overlay{Tier: "deep"}, nil); kept.Effort != "high" {
		t.Fatalf("a tier overlay dropped the effort: %+v", kept)
	}
}
