package policy_test

// Overlay dispatch producer — cycle-867 task overlay-dispatch-producer.
//
// overlays.go's resolver (ResolveOverlays/ResolveOverlaysWithAdvisor) has been
// fully implemented since cycle-609 but has zero non-test callers: nothing
// constructs an OverlayDispatch from live cycle dispatch data. This is RED —
// policy.DispatchFromPhaseRequest does not exist yet. It is a pure,
// side-effect-free field mapping (phase/cli/model/tier -> OverlayDispatch);
// the caller resolves the routing-mode tier logic BEFORE calling (empty tier
// for the non-auto degrade floor, populated tier only under
// model_routing=auto with a non-nil clamped plan — mirrors
// core.PhaseRequest.ModelRoutingCLI/ModelRoutingTier, cyclerun_dispatch.go).
// This function does not touch bridge.Engine.Launch or
// guards/integrity_surface.go's ProtectedSurfaceManifest — that wiring is an
// explicit out-of-cycle manual-ship carryover per the overlays.go header.

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// TestDispatchFromPhaseRequest_AutoTierPopulatedRoundTripsToFable: under
// model_routing=auto with a populated ModelRoutingTier ("deep"), the producer
// must carry phase/cli/model/tier through verbatim, and the resulting
// OverlayDispatch must resolve to ["fable"] via Policy.ResolveOverlays (the
// compiled default), proving the round trip end to end.
func TestDispatchFromPhaseRequest_AutoTierPopulatedRoundTripsToFable(t *testing.T) {
	d := policy.DispatchFromPhaseRequest("build", "claude-tmux", "opus", "deep")

	want := policy.OverlayDispatch{Phase: "build", CLI: "claude-tmux", Model: "opus", Tier: "deep"}
	if d != want {
		t.Fatalf("DispatchFromPhaseRequest(auto,deep) = %+v, want %+v", d, want)
	}

	var pol policy.Policy
	skills := pol.ResolveOverlays(d)
	if !reflect.DeepEqual(skills, []string{"fable"}) {
		t.Errorf("ResolveOverlays(round-tripped auto/deep dispatch) = %v, want [fable]", skills)
	}
}

// TestDispatchFromPhaseRequest_NonAutoTierEmptyRoundTripsToNoOverlay: outside
// model_routing=auto (or with no matching clamped-plan entry), the caller
// passes tier="" per the I4 degrade floor — the compiled default's
// tiers:[deep,top] selector must NOT match an empty tier, so the round trip
// resolves to zero skills, not "fable" (confirms the dormancy-to-live wiring
// doesn't silently activate fable outside auto mode).
func TestDispatchFromPhaseRequest_NonAutoTierEmptyRoundTripsToNoOverlay(t *testing.T) {
	d := policy.DispatchFromPhaseRequest("audit", "codex", "gpt-5", "")

	want := policy.OverlayDispatch{Phase: "audit", CLI: "codex", Model: "gpt-5", Tier: ""}
	if d != want {
		t.Fatalf("DispatchFromPhaseRequest(non-auto,empty-tier) = %+v, want %+v", d, want)
	}

	var pol policy.Policy
	skills := pol.ResolveOverlays(d)
	if len(skills) != 0 {
		t.Errorf("ResolveOverlays(round-tripped non-auto empty-tier dispatch) = %v, want empty (fable must not fire without auto tier)", skills)
	}
}

// TestDispatchFromPhaseRequest_PhasePassthrough: the phase name is carried
// through unchanged regardless of tier/cli/model — a plain field mapping, not
// a phase-conditioned transform.
func TestDispatchFromPhaseRequest_PhasePassthrough(t *testing.T) {
	cases := []string{"scout", "tdd", "build", "audit", "ship", "retro"}
	for _, phase := range cases {
		d := policy.DispatchFromPhaseRequest(phase, "claude-p", "sonnet", "top")
		if d.Phase != phase {
			t.Errorf("DispatchFromPhaseRequest(phase=%q).Phase = %q, want passthrough %q", phase, d.Phase, phase)
		}
	}
}
