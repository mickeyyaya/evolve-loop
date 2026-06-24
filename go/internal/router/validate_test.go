package router

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// TestValidatePlan_RejectsMalformedAndRegressive pins WS2-S1 (ADR-0052, research
// principle P6 "validate before clamp"): ValidatePlan is a pure, report-only
// pre-floor check. Case table — empty→reject, canonical→accept, unknown→reject,
// duplicate→reject, ship-while-skipping-audit→reject.
func TestValidatePlan_RejectsMalformedAndRegressive(t *testing.T) {
	t.Parallel()
	in := RouteInput{} // canonicalOrder is the known-set fallback when Cfg is empty

	if rej := ValidatePlan(in, &PhasePlan{}); len(rej) != 1 || rej[0].Reason != "empty-plan" {
		t.Errorf("empty plan: got %+v, want one empty-plan rejection", rej)
	}
	if rej := ValidatePlan(in, nil); len(rej) != 1 || rej[0].Reason != "empty-plan" {
		t.Errorf("nil plan: got %+v, want one empty-plan rejection", rej)
	}

	good := &PhasePlan{Entries: []PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "build", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
	if rej := ValidatePlan(in, good); len(rej) != 0 {
		t.Errorf("canonical plan must be accepted; got %+v", rej)
	}

	if rej := ValidatePlan(in, &PhasePlan{Entries: []PhasePlanEntry{{Phase: "frobnicate", Run: true}}}); !hasReason(rej, "unknown-phase") {
		t.Errorf("unknown phase must be rejected; got %+v", rej)
	}
	if rej := ValidatePlan(in, &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true}, {Phase: "build", Run: false}}}); !hasReason(rej, "duplicate-phase") {
		t.Errorf("duplicate phase must be rejected; got %+v", rej)
	}
	if rej := ValidatePlan(in, &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true}, {Phase: "ship", Run: true}}}); !hasReason(rej, "ship-skips-audit") {
		t.Errorf("ship without audit must be reported; got %+v", rej)
	}
}

// TestValidatePlan_DoesNotRejectFreshlyMintedPhase is the must-fix mint-aware
// case: a phase minted IN THIS PLAN is a known phase, never "unknown".
func TestValidatePlan_DoesNotRejectFreshlyMintedPhase(t *testing.T) {
	t.Parallel()
	plan := &PhasePlan{
		Entries:    []PhasePlanEntry{{Phase: "scout", Run: true}, {Phase: "novel-recon", Run: true}},
		MintPhases: []phaseconfig.PhaseConfig{{PhaseSpec: phasespec.PhaseSpec{Name: "novel-recon"}}},
	}
	for _, r := range ValidatePlan(RouteInput{}, plan) {
		if r.Phase == "novel-recon" {
			t.Errorf("a freshly-minted phase must NOT be flagged unknown: %+v", r)
		}
	}
}

// TestValidatePlan_DoesNotMutateOrWiden proves ValidatePlan is report-only — it
// never edits the plan or widens the run-set (the floor stays the sole disposer).
func TestValidatePlan_DoesNotMutateOrWiden(t *testing.T) {
	t.Parallel()
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "ship", Run: true}, {Phase: "frobnicate", Run: true}}}
	before := append([]PhasePlanEntry(nil), plan.Entries...)
	_ = ValidatePlan(RouteInput{}, plan)
	if !reflect.DeepEqual(plan.Entries, before) {
		t.Errorf("ValidatePlan mutated the plan:\nbefore=%+v\nafter =%+v", before, plan.Entries)
	}
}

func hasReason(rej []PlanRejection, reason string) bool {
	for _, r := range rej {
		if r.Reason == reason {
			return true
		}
	}
	return false
}
