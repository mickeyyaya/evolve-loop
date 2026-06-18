package ship

import (
	"os"
	"testing"
)

// shipControlFlowEnvVars are the env vars Run()/verify*/gitops read via
// os.Getenv that change ship's control flow. TestMain neutralizes them before
// the suite runs so the audit-binding matrix stays hermetic.
//
// Tests that exercise a specific var set it explicitly via Options.Env
// (highest precedence) or t.Setenv (which restores to the neutralized state
// on cleanup), so this composes cleanly with them.
var shipControlFlowEnvVars = []string{
	"EVOLVE_SHIP_AUTO_CONFIRM",
	"EVOLVE_STRICT_AUDIT",
	"EVOLVE_SHIP_RELEASE_NOTES",
	"EVOLVE_BYPASS_PREFIX_GATE",
}

func TestMain(m *testing.M) {
	for _, v := range shipControlFlowEnvVars {
		os.Unsetenv(v)
	}
	os.Exit(m.Run())
}

// TestShipSuiteIsHermetic asserts TestMain neutralized the control-flow vars,
// so the suite reflects ship's logic rather than the operator's shell. Fails
// loudly if TestMain is removed or a new control-flow var is added unlisted.
func TestShipSuiteIsHermetic(t *testing.T) {
	for _, v := range shipControlFlowEnvVars {
		if got := os.Getenv(v); got != "" {
			t.Errorf("%s=%q leaked into the ship test process; TestMain must neutralize all control-flow vars so audit-binding tests aren't vacuously bypassed", v, got)
		}
	}
}
