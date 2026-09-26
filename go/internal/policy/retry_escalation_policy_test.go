package policy

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
