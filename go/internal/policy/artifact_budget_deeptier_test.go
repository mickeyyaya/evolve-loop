package policy_test

// artifact_budget_deeptier_test.go — the compiled deep-tier artifact budgets.
//
// Incident (inbox item deep-phase-artifact-budget-too-small, P1/0.95): SIX
// phases died with codes=[missing_artifact] — report + acs absent, a
// content-free infra FAIL that burns the whole cycle — across FOUR phase types
// in one day: audit ×2 (cycle-1201), retro ×1 (1201), adversarial-review ×1
// (1217), tdd ×2 (1218, 1219). The last landed on a QUIET host (load 3.0, zero
// leaked processes), which rules out contention as the sole cause: load made it
// worse, the BUDGET is the floor.
//
// The arithmetic: the bridge artifact-wait base is 300s
// (bridge.tmuxArtifactTimeoutS) and the deterministic reviewer grants at most 6
// extends (bridge.defaultArtifactMaxExtends) ⇒ ~650s of effective wall clock
// before exit 81. Before this fix, defaultPhaseArtifactTimeoutS raised ONLY
// retrospective/retro, so every deep-tier (opus) phase doing real analysis on
// this repo was one slow turn from a lost cycle at ~11 minutes.
//
// An operator-side .evolve/policy.json block mitigates a configured checkout.
// These predicates pin the COMPILED defaults, i.e. the contract for a FRESH
// CLONE, which the operator block cannot reach.
//
// The two invariants that must survive together:
//  1. the four deep-tier phase labels carry 1200s;
//  2. every OTHER phase still resolves 0 — the bridge's "use the builtin"
//     sentinel — because global hang detection must not be broadly weakened to
//     fix the phases that legitimately think for a long time.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// deepTierBudgetS is the compiled budget for a deep-tier analysis phase.
// 1200s × (1 + 6 extends) is the effective ceiling; the value is the StopReviewer
// REVIEW INTERVAL, not a hard deadline.
const deepTierBudgetS = 1200

// TestBridgePolicy_DeepTierArtifactBudgets pins requirement (1): a zero-value
// BridgePolicy — the REAL state of a fresh clone, whose checked-in
// .evolve/policy.json carries no bridge block — must already resolve the
// deep-tier budgets. Both vocabularies are asserted for the same reason the
// retro entry carries two keys (the cycle-1054 unit-green/live-dead defect): the
// runner dispatches Agent = the core phase NAME ("tdd", "build", "audit"), while
// the persona/agent vocabulary spells the same phases "tdd-engineer", "builder",
// "auditor". Carrying both makes a rename on either side harmless instead of
// silently restoring the 300s cliff.
func TestBridgePolicy_DeepTierArtifactBudgets(t *testing.T) {
	got := policy.BridgePolicy{}.PhaseArtifactTimeouts()
	for _, label := range []string{
		"tdd", "tdd-engineer",
		"build", "builder",
		"audit", "auditor",
		"adversarial-review",
	} {
		if got[label] != deepTierBudgetS {
			t.Errorf("compiled default for %q = %d, want %d — a FRESH CLONE must not have the ~650s cliff "+
				"that cost six cycles in one day; an operator policy.json block cannot fix a clone",
				label, got[label], deepTierBudgetS)
		}
	}
	// retro keeps its own, smaller budget: its contract grew, but it is not a
	// deep-tier analysis phase and 900s has held since cycle-1048.
	for _, label := range []string{"retrospective", "retro"} {
		if got[label] != 900 {
			t.Errorf("compiled default for %q = %d, want 900 — the retro budget is unchanged by this fix",
				label, got[label])
		}
	}
}

// TestBridgePolicy_NonDeepPhasesKeepBuiltinSentinel pins requirement (2), the
// design intent stated in defaultPhaseArtifactTimeoutS's own comment: this is a
// NARROW list, not a blanket raise. A phase absent from the map resolves 0 —
// which the bridge reads as "use the 300s builtin" — so a wedged scout/intent/
// triage/ship is still caught in ~11 minutes rather than ~2.3 hours.
func TestBridgePolicy_NonDeepPhasesKeepBuiltinSentinel(t *testing.T) {
	got := policy.BridgePolicy{}.PhaseArtifactTimeouts()
	for _, label := range []string{
		"scout", "intent", "triage", "ship", "memo",
		"plan-review", "build-planner", "debugger", "evaluate",
		"", "not-a-phase",
	} {
		if v := got[label]; v != 0 {
			t.Errorf("phase %q resolved %d, want 0 — only the deep-tier analysis phases are widened; "+
				"global hang detection must not be broadly weakened", label, v)
		}
	}
}

// TestBridgePolicy_DeepTierOverrideSemantics proves the merge rules still hold
// over the NEW compiled entries, not just over retro: a positive operator value
// wins (in either direction — explicit config is authoritative), while a zero or
// negative entry is rejected rather than applied, so an operator typo can never
// re-open the cliff or produce a negative deadline.
func TestBridgePolicy_DeepTierOverrideSemantics(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    map[string]int
		phase string
		want  int
	}{
		{"raise-wins", map[string]int{"audit": 1800}, "audit", 1800},
		{"lower-wins-explicit-config-is-authoritative", map[string]int{"tdd": 400}, "tdd", 400},
		{"zero-rejected", map[string]int{"build": 0}, "build", deepTierBudgetS},
		{"negative-rejected", map[string]int{"adversarial-review": -1}, "adversarial-review", deepTierBudgetS},
		{"large-negative-rejected", map[string]int{"builder": -100000}, "builder", deepTierBudgetS},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.BridgePolicy{PhaseArtifactTimeoutS: tc.in}.PhaseArtifactTimeouts()
			if got[tc.phase] != tc.want {
				t.Errorf("PhaseArtifactTimeouts()[%q] = %d, want %d", tc.phase, got[tc.phase], tc.want)
			}
		})
	}

	// An override of one deep-tier phase must not disturb its siblings, and must
	// not bleed into the global artifact_timeout_s budget.
	bp := policy.BridgePolicy{PhaseArtifactTimeoutS: map[string]int{"audit": 1800}}
	got := bp.PhaseArtifactTimeouts()
	if got["tdd"] != deepTierBudgetS {
		t.Errorf("sibling tdd = %d after an audit override, want %d", got["tdd"], deepTierBudgetS)
	}
	if bp.ArtifactTimeoutS != 0 {
		t.Errorf("global ArtifactTimeoutS = %d, want 0 (per-phase must never bleed into global)", bp.ArtifactTimeoutS)
	}

	// Fresh map per call: a caller mutating a resolved deep-tier entry must not
	// poison the next resolution.
	first := policy.BridgePolicy{}.PhaseArtifactTimeouts()
	first["audit"] = 1
	if second := (policy.BridgePolicy{}).PhaseArtifactTimeouts()["audit"]; second != deepTierBudgetS {
		t.Errorf("after caller mutation, fresh resolve gave audit = %d, want %d — the resolver must not "+
			"alias the package-level map", second, deepTierBudgetS)
	}
}
