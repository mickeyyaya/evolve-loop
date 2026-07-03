package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Cycle-480 Task 1 (universal-envelope-floor) RED tests.
//
// Root cause (scout Key Finding 3): the operator low-model floor at
// model_routing_clamp.go:52 gates the clamp-up entirely on
// `prof.ModelTierEnvelope != nil`. 72/91 profiles declare NO envelope, so a
// `tier:fast` advisor proposal against a nil-envelope profile falls through to
// policy.ValidatePin, which (by design B2) treats "no envelope configured" as a
// PREFERENCE, not a violation — and is therefore NEVER clamped up to the
// balanced floor. The fix substitutes a compiled-default envelope
// {min:balanced, max:deep} at the clamp site when ModelTierEnvelope == nil, so
// every phase — regardless of whether its profile happens to declare an
// envelope — gets the same universal floor.
//
// These tests exercise the SUT (ClampPlanModelRouting) directly. Builder must
// NOT modify this file — implement the production fix in
// model_routing_clamp.go until they are GREEN.

// TestClampPlanModelRouting_NilEnvelopeFloorClampsUp (Task1-AC1, behavioral +
// positive): a below-floor tier ("fast") proposed against a profile with NO
// declared envelope must be clamped UP to the universal default floor
// ("balanced"), with the CLI left untouched and exactly one
// "model-routing-guardrail" clamp recorded. RED today: the nil-envelope profile
// skips the clamp-up gate and ValidatePin honors fast as a preference (tier
// stays "fast", zero clamps).
func TestClampPlanModelRouting_NilEnvelopeFloorClampsUp(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux"} // NO ModelTierEnvelope declared
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "claude", Tier: "fast"}}}

	// nil catalogLookup isolates the envelope-floor behavior from the
	// catalog-resolvability gate.
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

// TestClampPlanModelRouting_NilEnvelopeFloorAppliesAcrossPhases (Task1-AC4,
// anti-gaming + semantic): the universal floor must apply to EVERY phase, not a
// single hardcoded phase name. A builder who hardcodes the floor to fire only
// for phase "build" (matching AC1) would pass AC1 but fail here. Two distinct
// nil-envelope phases both proposing "fast" must BOTH be clamped up. RED today
// (no default floor exists at all).
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

// TestClampPlanModelRouting_ExplicitEnvelopeNotOverriddenByDefault (Task1-AC2,
// negative boundary): a profile that EXPLICITLY declares an envelope permitting
// "fast" (Min:fast) must keep its proposal unclamped — the compiled default
// floor applies ONLY when no envelope is declared. A buggy fix that applies the
// balanced default unconditionally (clobbering explicit envelopes) turns this
// RED. Correct today and must stay GREEN.
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

// TestClampPlanModelRouting_NilEnvelopeWithinCeilingPassesThrough (Task1-AC3,
// edge boundary): a nil-envelope proposal already AT/above the floor and within
// the default ceiling ("deep", the default Max) passes through unclamped. A
// buggy fix that clamps every nil-envelope tier down/up to the floor would turn
// this RED. Correct today and must stay GREEN.
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
