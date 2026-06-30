package policy_test

// parallel_evaluate_config_test.go — resolution tests for policy.ParallelEvaluatePolicy
// and Policy.ParallelEvaluateConfig() (mirrors mergegate_config_test.go, T1 RED).
//
// RED: policy.ParallelEvaluatePolicy and Policy.ParallelEvaluateConfig() do not yet
// exist; this file fails to compile until Builder adds them (compile-fail = RED evidence).

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestParallelEvaluateConfig_AbsentDefaults(t *testing.T) {
	got := policy.Policy{}.ParallelEvaluateConfig()
	if got.Stage != "off" {
		t.Errorf("absent block: Stage = %q, want %q", got.Stage, "off")
	}
	if got.Concurrency != 3 {
		t.Errorf("absent block: Concurrency = %d, want 3 (soak sweet spot)", got.Concurrency)
	}
}

func TestParallelEvaluateConfig_EmptyBlockDefaults(t *testing.T) {
	got := policy.Policy{ParallelEvaluate: &policy.ParallelEvaluatePolicy{}}.ParallelEvaluateConfig()
	if got.Stage != "off" {
		t.Errorf("empty block: Stage = %q, want %q", got.Stage, "off")
	}
	if got.Concurrency != 3 {
		t.Errorf("empty block: Concurrency = %d, want 3", got.Concurrency)
	}
}

func TestParallelEvaluateConfig_StageOverrideShadow(t *testing.T) {
	got := policy.Policy{ParallelEvaluate: &policy.ParallelEvaluatePolicy{Stage: "shadow"}}.ParallelEvaluateConfig()
	if got.Stage != "shadow" {
		t.Errorf("stage=shadow: got %q, want %q", got.Stage, "shadow")
	}
}

func TestParallelEvaluateConfig_StageOverrideEnforce(t *testing.T) {
	got := policy.Policy{ParallelEvaluate: &policy.ParallelEvaluatePolicy{Stage: "enforce"}}.ParallelEvaluateConfig()
	if got.Stage != "enforce" {
		t.Errorf("stage=enforce: got %q, want %q", got.Stage, "enforce")
	}
}

// TestParallelEvaluateConfig_UnknownStageFallsToOff is the load-bearing negative test:
// a typo (e.g. "enableddddd") must map to "off", never to "enforce" or any other
// active stage, so a misspelling cannot silently arm the parallel dispatcher.
func TestParallelEvaluateConfig_UnknownStageFallsToOff(t *testing.T) {
	got := policy.Policy{ParallelEvaluate: &policy.ParallelEvaluatePolicy{Stage: "enableddddd"}}.ParallelEvaluateConfig()
	if got.Stage != "off" {
		t.Errorf("unknown stage: got %q, want %q (fail-safe must never activate)", got.Stage, "off")
	}
}

func TestParallelEvaluateConfig_ZeroConcurrencyDefaultsTo3(t *testing.T) {
	got := policy.Policy{ParallelEvaluate: &policy.ParallelEvaluatePolicy{Concurrency: 0}}.ParallelEvaluateConfig()
	if got.Concurrency != 3 {
		t.Errorf("zero concurrency: got %d, want 3 (zero must not produce a useless dispatcher)", got.Concurrency)
	}
}

func TestParallelEvaluateConfig_NegativeConcurrencyDefaultsTo3(t *testing.T) {
	got := policy.Policy{ParallelEvaluate: &policy.ParallelEvaluatePolicy{Concurrency: -5}}.ParallelEvaluateConfig()
	if got.Concurrency != 3 {
		t.Errorf("negative concurrency: got %d, want 3", got.Concurrency)
	}
}

func TestParallelEvaluateConfig_FullOverride(t *testing.T) {
	got := policy.Policy{ParallelEvaluate: &policy.ParallelEvaluatePolicy{Stage: "enforce", Concurrency: 5}}.ParallelEvaluateConfig()
	if got.Stage != "enforce" {
		t.Errorf("full override stage: got %q, want %q", got.Stage, "enforce")
	}
	if got.Concurrency != 5 {
		t.Errorf("full override concurrency: got %d, want 5", got.Concurrency)
	}
}

func TestParallelEvaluateConfig_ConcurrencyOverridePositive(t *testing.T) {
	got := policy.Policy{ParallelEvaluate: &policy.ParallelEvaluatePolicy{Concurrency: 6}}.ParallelEvaluateConfig()
	if got.Concurrency != 6 {
		t.Errorf("concurrency override: got %d, want 6", got.Concurrency)
	}
}
