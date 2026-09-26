package policy_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const deepTierBudgetS = 1200

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
	for _, label := range []string{"retrospective", "retro"} {
		if got[label] != 900 {
			t.Errorf("compiled default for %q = %d, want 900 — the retro budget is unchanged by this fix",
				label, got[label])
		}
	}
}

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

	bp := policy.BridgePolicy{PhaseArtifactTimeoutS: map[string]int{"audit": 1800}}
	got := bp.PhaseArtifactTimeouts()
	if got["tdd"] != deepTierBudgetS {
		t.Errorf("sibling tdd = %d after an audit override, want %d", got["tdd"], deepTierBudgetS)
	}
	if bp.ArtifactTimeoutS != 0 {
		t.Errorf("global ArtifactTimeoutS = %d, want 0 (per-phase must never bleed into global)", bp.ArtifactTimeoutS)
	}

	first := policy.BridgePolicy{}.PhaseArtifactTimeouts()
	first["audit"] = 1
	if second := (policy.BridgePolicy{}).PhaseArtifactTimeouts()["audit"]; second != deepTierBudgetS {
		t.Errorf("after caller mutation, fresh resolve gave audit = %d, want %d — the resolver must not "+
			"alias the package-level map", second, deepTierBudgetS)
	}
}
