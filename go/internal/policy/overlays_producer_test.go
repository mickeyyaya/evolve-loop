package policy_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestDispatchFromPhaseRequest_AutoTierPopulatedRoundTripsToFable(t *testing.T) {
	d := policy.DispatchFromPhaseRequest("build", "claude-tmux", "opus", "deep")

	want := policy.OverlayDispatch{Phase: "build", CLI: "claude-tmux", Model: "opus", Tier: "deep"}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("DispatchFromPhaseRequest(auto,deep) = %+v, want %+v", d, want)
	}

	var pol policy.Policy
	skills := pol.ResolveOverlays(d)
	if !reflect.DeepEqual(skills, []string{"fable"}) {
		t.Errorf("ResolveOverlays(round-tripped auto/deep dispatch) = %v, want [fable]", skills)
	}
}

func TestDispatchFromPhaseRequest_NonAutoTierEmptyRoundTripsToNoOverlay(t *testing.T) {
	d := policy.DispatchFromPhaseRequest("audit", "codex", "gpt-5", "")

	want := policy.OverlayDispatch{Phase: "audit", CLI: "codex", Model: "gpt-5", Tier: ""}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("DispatchFromPhaseRequest(non-auto,empty-tier) = %+v, want %+v", d, want)
	}

	var pol policy.Policy
	skills := pol.ResolveOverlays(d)
	if len(skills) != 0 {
		t.Errorf("ResolveOverlays(round-tripped non-auto empty-tier dispatch) = %v, want empty (fable must not fire without auto tier)", skills)
	}
}

func TestDispatchFromPhaseRequest_PhasePassthrough(t *testing.T) {
	cases := []string{"scout", "tdd", "build", "audit", "ship", "retro"}
	for _, phase := range cases {
		d := policy.DispatchFromPhaseRequest(phase, "claude-p", "sonnet", "top")
		if d.Phase != phase {
			t.Errorf("DispatchFromPhaseRequest(phase=%q).Phase = %q, want passthrough %q", phase, d.Phase, phase)
		}
	}
}
