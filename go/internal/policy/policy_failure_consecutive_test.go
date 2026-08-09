package policy

import "testing"

// Pins the compiled default for the consecutive-failures halt (operator
// directive 2026-08-10) and its per-threshold merge override.

func TestDefaultConsecutiveFailuresHaltCeilingIsThree(t *testing.T) {
	if got := DefaultSystemFailurePolicy().Thresholds.ConsecutiveFailuresHaltCeiling; got != 3 {
		t.Fatalf("compiled default ConsecutiveFailuresHaltCeiling = %d, want 3", got)
	}
}

func TestConsecutiveFailuresHaltCeilingMergesFromOperatorPolicy(t *testing.T) {
	p := Policy{SystemFailurePolicy: &SystemFailurePolicy{Thresholds: FailureThresholds{ConsecutiveFailuresHaltCeiling: 5}}}
	cfg, err := p.FailurePolicyConfig()
	if err != nil {
		t.Fatalf("FailurePolicyConfig: %v", err)
	}
	if cfg.Thresholds.ConsecutiveFailuresHaltCeiling != 5 {
		t.Errorf("operator override = %d, want 5", cfg.Thresholds.ConsecutiveFailuresHaltCeiling)
	}
	if cfg.Thresholds.IdenticalFingerprintHaltCeiling != 3 {
		t.Errorf("unset sibling threshold must keep compiled default 3, got %d", cfg.Thresholds.IdenticalFingerprintHaltCeiling)
	}
}
