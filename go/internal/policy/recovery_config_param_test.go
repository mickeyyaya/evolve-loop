package policy_test

// RecoveryPolicy — the ADR-0044 Unified Phase Recovery config (cycle-12 flag retirement).
// PhaseRecovery defaults "shadow" (behavior-neutral); absent block is safe.

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
			policy.RecoveryPolicy{PhaseRecovery: "shadow"},
		},
		{
			"empty-block-defaults-shadow",
			policy.Policy{Recovery: &policy.RecoveryPolicy{}},
			policy.RecoveryPolicy{PhaseRecovery: "shadow"},
		},
		{
			"enforce-set",
			policy.Policy{Recovery: &policy.RecoveryPolicy{PhaseRecovery: "enforce"}},
			policy.RecoveryPolicy{PhaseRecovery: "enforce"},
		},
		{
			"off-set",
			policy.Policy{Recovery: &policy.RecoveryPolicy{PhaseRecovery: "off"}},
			policy.RecoveryPolicy{PhaseRecovery: "off"},
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
			policy.RecoveryPolicy{PhaseRecovery: "shadow"},
		},
		{
			"enforce-set",
			`{"recovery":{"phase_recovery":"enforce"}}`,
			policy.RecoveryPolicy{PhaseRecovery: "enforce"},
		},
		{
			"shadow-explicit",
			`{"recovery":{"phase_recovery":"shadow"}}`,
			policy.RecoveryPolicy{PhaseRecovery: "shadow"},
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
