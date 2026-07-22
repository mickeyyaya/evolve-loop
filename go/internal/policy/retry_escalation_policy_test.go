package policy

// retry_escalation_policy_test.go — ADR-0076 slice D policy knob: the
// failure-count threshold at which a retried item's build escalates to deep.
// Compiled default 1 (first retry escalates); positive-override merge (the
// TaskRetryCeiling idiom — 0 keeps the default, documented convention).

import "testing"

func TestBuildDeepEscalateAtFailures_CompiledDefault(t *testing.T) {
	got := DefaultSystemFailurePolicy().Thresholds.BuildDeepEscalateAtFailures
	if got != 1 {
		t.Fatalf("compiled default must be 1 (first retry escalates), got %d", got)
	}
}

func TestBuildDeepEscalateAtFailures_PositiveOverrideMerge(t *testing.T) {
	p := Policy{SystemFailurePolicy: &SystemFailurePolicy{
		Thresholds: FailureThresholds{BuildDeepEscalateAtFailures: 3},
	}}
	cfg, err := p.FailurePolicyConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Thresholds.BuildDeepEscalateAtFailures != 3 {
		t.Fatalf("positive override must win, got %d", cfg.Thresholds.BuildDeepEscalateAtFailures)
	}
	p2 := Policy{SystemFailurePolicy: &SystemFailurePolicy{Thresholds: FailureThresholds{}}}
	cfg2, _ := p2.FailurePolicyConfig()
	if cfg2.Thresholds.BuildDeepEscalateAtFailures != 1 {
		t.Fatalf("zero keeps compiled default, got %d", cfg2.Thresholds.BuildDeepEscalateAtFailures)
	}
}
