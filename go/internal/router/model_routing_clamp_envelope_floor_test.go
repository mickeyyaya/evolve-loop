package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestClampPlanModelRouting_NilEnvelopeFloorClampsUp(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux"} // NO ModelTierEnvelope declared
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "claude", Tier: "fast"}}}

	// A nil catalogLookup isolates the envelope floor from the catalog check.
	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), nil)

	if len(clamps) != 1 {
		t.Fatalf("clamps = %+v, want exactly one clamp-up (nil-envelope profile must still get the universal balanced floor)", clamps)
	}
	if clamps[0].Rule != "model-routing-guardrail" {
		t.Errorf("clamp Rule = %q, want %q", clamps[0].Rule, "model-routing-guardrail")
	}
	if got := out.Entries[0].Tier; policy.TierRank(got) != policy.TierRank("balanced") {
		t.Errorf("tier clamped to %q, want balanced-rank (the universal floor)", got)
	}
	if out.Entries[0].CLI != "claude" {
		t.Errorf("CLI mutated to %q, want unchanged claude (only the tier violated the floor)", out.Entries[0].CLI)
	}
}

func TestClampPlanModelRouting_NilEnvelopeFloorAppliesAcrossPhases(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux"} // nil envelope for all phases
	plan := &PhasePlan{Entries: []PhasePlanEntry{
		{Phase: "scout", Run: true, CLI: "claude", Tier: "fast"},
		{Phase: "audit", Run: true, CLI: "claude", Tier: "fast"},
	}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), nil)

	if len(clamps) != 2 {
		t.Fatalf("clamps = %+v, want two (the floor is universal, not hardcoded to one phase)", clamps)
	}
	for i, e := range out.Entries {
		if policy.TierRank(e.Tier) != policy.TierRank("balanced") {
			t.Errorf("entry[%d] phase %q tier = %q, want clamped up to balanced", i, e.Phase, e.Tier)
		}
	}
}

func TestClampPlanModelRouting_ExplicitEnvelopeNotOverriddenByDefault(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux",
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "deep"}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "claude", Tier: "fast"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), nil)

	if len(clamps) != 0 {
		t.Fatalf("clamps = %+v, want none (explicit envelope permits fast; default floor must not override it)", clamps)
	}
	if out.Entries[0].Tier != "fast" {
		t.Errorf("tier forced to %q, want unchanged fast (explicit envelope honored)", out.Entries[0].Tier)
	}
}

func TestClampPlanModelRouting_NilEnvelopeWithinCeilingPassesThrough(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux"} // nil envelope
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "claude", Tier: "deep"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), nil)

	if len(clamps) != 0 {
		t.Fatalf("clamps = %+v, want none (deep is within the default ceiling)", clamps)
	}
	if out.Entries[0].Tier != "deep" {
		t.Errorf("tier forced to %q, want unchanged deep", out.Entries[0].Tier)
	}
}
