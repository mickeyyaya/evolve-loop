package router

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

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
