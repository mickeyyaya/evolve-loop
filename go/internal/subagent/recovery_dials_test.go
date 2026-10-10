package subagent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestExecAdapterDeps_CarriesThePolicyRecoveryDials(t *testing.T) {
	cases := []struct {
		name, policy            string
		wantRecovery, wantFatal config.Stage
	}{
		{"no-policy-file-compiled-defaults", "", config.StageShadow, config.StageEnforce},
		{"fatal-pane-escape-hatch", `{"recovery":{"fatal_pane":"shadow"}}`, config.StageShadow, config.StageShadow},
		{"program-dial-is-independent", `{"recovery":{"phase_recovery":"enforce"}}`, config.StageEnforce, config.StageEnforce},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.policy != "" {
				if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(tc.policy), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			d := execAdapterDeps(map[string]string{"HOME": t.TempDir(), "EVOLVE_PROJECT_ROOT": root})
			if d.RecoveryStage != tc.wantRecovery.String() || d.FatalPaneStage != tc.wantFatal.String() {
				t.Errorf("execAdapterDeps: RecoveryStage=%q FatalPaneStage=%q, want %q/%q",
					d.RecoveryStage, d.FatalPaneStage, tc.wantRecovery.String(), tc.wantFatal.String())
			}
		})
	}
}

func TestExecAdapterDeps_NoProjectRootResolvesCompiledDefaults(t *testing.T) {
	d := execAdapterDeps(map[string]string{"HOME": t.TempDir()})
	if d.RecoveryStage != config.StageShadow.String() || d.FatalPaneStage != config.StageEnforce.String() {
		t.Errorf("execAdapterDeps without a project root: %q/%q, want shadow/enforce", d.RecoveryStage, d.FatalPaneStage)
	}
}
