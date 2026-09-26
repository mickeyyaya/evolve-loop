package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestSetFatalPaneStage_WiresField(t *testing.T) {
	a := New()
	a.SetRecoveryStage("shadow")
	for _, stage := range []string{"", "shadow", "enforce", "off"} {
		a.SetFatalPaneStage(stage)
		if a.fatalPaneStage != stage {
			t.Errorf("SetFatalPaneStage(%q) → a.fatalPaneStage = %q, want %q", stage, a.fatalPaneStage, stage)
		}
		if a.recoveryStage != "shadow" {
			t.Fatalf("SetFatalPaneStage(%q) moved the program dial: recoveryStage = %q", stage, a.recoveryStage)
		}
	}
}

func TestProductionEngineDeps_CarriesBothRecoveryDials(t *testing.T) {
	a := New()
	a.SetRecoveryStage("shadow")
	a.SetFatalPaneStage("enforce")
	d := a.productionEngineDeps(nil)
	if d.RecoveryStage != "shadow" || d.FatalPaneStage != "enforce" {
		t.Fatalf("productionEngineDeps: RecoveryStage=%q FatalPaneStage=%q, want shadow/enforce", d.RecoveryStage, d.FatalPaneStage)
	}
}

func TestNewDefault_SeedsRecoveryDialsFromPolicy(t *testing.T) {
	cases := []struct {
		name, policy            string
		wantRecovery, wantFatal config.Stage
	}{
		{"absent-policy-compiled-defaults", "", config.StageShadow, config.StageEnforce},
		{"empty-recovery-block-compiled-defaults", `{"recovery":{}}`, config.StageShadow, config.StageEnforce},
		{"fatal-pane-escape-hatch", `{"recovery":{"fatal_pane":"shadow"}}`, config.StageShadow, config.StageShadow},
		{"program-dial-does-not-move-fatal-pane", `{"recovery":{"phase_recovery":"off"}}`, config.StageOff, config.StageEnforce},
		{"mis-cased-word-is-off-like-the-cycle-root", `{"recovery":{"fatal_pane":"Enforce"}}`, config.StageShadow, config.StageOff},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.policy != "" {
				writePolicyJson(t, dir, tc.policy)
			}
			a := NewDefault(dir, nil)
			if a.recoveryStage != tc.wantRecovery.String() || a.fatalPaneStage != tc.wantFatal.String() {
				t.Errorf("NewDefault: recoveryStage=%q fatalPaneStage=%q, want %q/%q",
					a.recoveryStage, a.fatalPaneStage, tc.wantRecovery.String(), tc.wantFatal.String())
			}
		})
	}
}
