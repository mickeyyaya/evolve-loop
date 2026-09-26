package policy_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
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
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce", FatalPane: "enforce"},
		},
		{
			"empty-block-defaults-shadow",
			policy.Policy{Recovery: &policy.RecoveryPolicy{}},
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce", FatalPane: "enforce"},
		},
		{
			"enforce-set",
			policy.Policy{Recovery: &policy.RecoveryPolicy{PhaseRecovery: "enforce", SpineFloor: "enforce"}},
			policy.RecoveryPolicy{PhaseRecovery: "enforce", SpineFloor: "enforce", FatalPane: "enforce"},
		},
		{
			"off-set",
			policy.Policy{Recovery: &policy.RecoveryPolicy{PhaseRecovery: "off", SpineFloor: "enforce"}},
			policy.RecoveryPolicy{PhaseRecovery: "off", SpineFloor: "enforce", FatalPane: "enforce"},
		},
		{
			"fatal-pane-shadow-set",
			policy.Policy{Recovery: &policy.RecoveryPolicy{FatalPane: "shadow"}},
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce", FatalPane: "shadow"},
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
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce", FatalPane: "enforce"},
		},
		{
			"enforce-set",
			`{"recovery":{"phase_recovery":"enforce"}}`,
			policy.RecoveryPolicy{PhaseRecovery: "enforce", SpineFloor: "enforce", FatalPane: "enforce"},
		},
		{
			"shadow-explicit",
			`{"recovery":{"phase_recovery":"shadow"}}`,
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce", FatalPane: "enforce"},
		},
		{
			"spine-floor-shadow-escape-hatch",
			`{"recovery":{"spine_floor":"shadow"}}`,
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "shadow", FatalPane: "enforce"},
		},
		{
			"fatal-pane-shadow-escape-hatch",
			`{"recovery":{"fatal_pane":"shadow"}}`,
			policy.RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce", FatalPane: "shadow"},
		},
		{
			"phase-recovery-off-leaves-fatal-pane",
			`{"recovery":{"phase_recovery":"off"}}`,
			policy.RecoveryPolicy{PhaseRecovery: "off", SpineFloor: "enforce", FatalPane: "enforce"},
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

func TestBridgeRecoveryStages_ParsesThroughTheLoaderTrichotomy(t *testing.T) {
	cases := []struct {
		name                    string
		rec                     *policy.RecoveryPolicy
		wantRecovery, wantFatal config.Stage
	}{
		{"absent-compiled-defaults", nil, config.StageShadow, config.StageEnforce},
		{"escape-hatches", &policy.RecoveryPolicy{PhaseRecovery: "off", FatalPane: "shadow"}, config.StageOff, config.StageShadow},
		{"case-is-not-folded", &policy.RecoveryPolicy{FatalPane: "Enforce"}, config.StageShadow, config.StageOff},
		{"typo-is-off", &policy.RecoveryPolicy{PhaseRecovery: "enforec", FatalPane: "shadwo"}, config.StageOff, config.StageOff},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, f := policy.Policy{Recovery: tc.rec}.BridgeRecoveryStages()
			if r != tc.wantRecovery.String() || f != tc.wantFatal.String() {
				t.Errorf("BridgeRecoveryStages() = %q/%q, want %q/%q", r, f, tc.wantRecovery.String(), tc.wantFatal.String())
			}
		})
	}
}
