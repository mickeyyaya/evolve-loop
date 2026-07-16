package policy_test

// RecoveryPolicy — the ADR-0044 Unified Phase Recovery config (cycle-12 flag retirement).
// PhaseRecovery defaults "shadow" (behavior-neutral); absent block is safe.
// R8.5 (2026-07-16): SpineFloor — the spine floor's OWN dial — defaults
// "enforce" (replay-evidenced flip); "shadow" is the policy escape hatch.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRecoveryConfig_Resolution(t *testing.T) {
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.RecoveryPolicy
	}{
		{
			"absent-defaults-shadow",
			policy.Policy{},
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce"},
		},
		{
			"empty-block-defaults-shadow",
			policy.Policy{Recovery: &policy.RecoveryPolicy{}},
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce"},
		},
		{
			"enforce-set",
			policy.Policy{Recovery: &policy.RecoveryPolicy{PhaseRecovery: "enforce", SpineFloor: "enforce"}},
			policy.RecoveryPolicy{PhaseRecovery: "enforce", SpineFloor: "enforce"},
		},
		{
			"off-set",
			policy.Policy{Recovery: &policy.RecoveryPolicy{PhaseRecovery: "off", SpineFloor: "enforce"}},
			policy.RecoveryPolicy{PhaseRecovery: "off", SpineFloor: "enforce"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pol.RecoveryConfig()
			if got != tc.want {
				t.Errorf("RecoveryConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLoad_RecoveryBlock(t *testing.T) {
	cases := []struct {
		name string
		json string
		want policy.RecoveryPolicy
	}{
		{
			"absent-block-defaults-shadow",
			`{}`,
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce"},
		},
		{
			"enforce-set",
			`{"recovery":{"phase_recovery":"enforce"}}`,
			policy.RecoveryPolicy{PhaseRecovery: "enforce", SpineFloor: "enforce"},
		},
		{
			"shadow-explicit",
			`{"recovery":{"phase_recovery":"shadow"}}`,
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce"},
		},
		{
			// The R8.5 escape hatch: dial the spine floor back to shadow via
			// policy.json, no recompile, without touching phase_recovery.
			"spine-floor-shadow-escape-hatch",
			`{"recovery":{"spine_floor":"shadow"}}`,
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "shadow"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol, err := policy.Load(writeTempPolicy(t, tc.json))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := pol.RecoveryConfig(); got != tc.want {
				t.Errorf("after Load, RecoveryConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
