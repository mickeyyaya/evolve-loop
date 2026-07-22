package policy

// size_budget_policy_test.go — ADR-0076 slice A: cycle-size → budget
// multipliers (compiled defaults, per-key positive-override merge — the
// PhaseArtifactTimeouts idiom). Consumed by the correction-limit and build
// artifact-timeout scaling.

import "testing"

func TestSizeBudgetMultipliers_CompiledDefaults(t *testing.T) {
	m := Policy{}.WorkflowConfig().SizeBudgetMultipliers
	want := map[string]float64{"trivial": 1.0, "small": 1.0, "medium": 1.25, "large": 1.5}
	for k, v := range want {
		if m[k] != v {
			t.Fatalf("default %s: want %v got %v", k, v, m[k])
		}
	}
}

func TestSizeBudgetMultipliers_PerKeyPositiveOverrideMerge(t *testing.T) {
	p := Policy{Workflow: &WorkflowPolicy{SizeBudgetMultipliers: map[string]float64{"large": 2.0}}}
	m := p.WorkflowConfig().SizeBudgetMultipliers
	if m["large"] != 2.0 {
		t.Fatalf("override must win for its key, got %v", m["large"])
	}
	if m["medium"] != 1.25 {
		t.Fatalf("unmentioned keys keep compiled defaults, got %v", m["medium"])
	}
}

func TestMaxContractCorrectionRetries_Exported(t *testing.T) {
	if MaxContractCorrectionRetries != 5 {
		t.Fatalf("exported clamp ceiling must match the resolver's, got %d", MaxContractCorrectionRetries)
	}
}
