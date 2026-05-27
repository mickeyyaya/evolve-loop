package guards

import (
	"os"
	"testing"
)

// bypassEnvVars are every gate-bypass / allow override the guards package
// reads via os.Getenv. The list MUST stay in sync with the envBypass() call
// sites across this package (role.go, ship.go, phase.go, docdelete.go,
// quota.go). A stray entry is harmless; a missing one re-opens the leak.
var bypassEnvVars = []string{
	"EVOLVE_BYPASS_ROLE_GATE",
	"EVOLVE_BYPASS_SHIP_GATE",
	"EVOLVE_BYPASS_PHASE_GATE",
	"EVOLVE_ALLOW_DOC_DELETE",
	"EVOLVE_ALLOW_DEEP_RESEARCH",
}

// TestMain neutralizes the operator's ambient bypass env vars before any test
// in this package runs. Without this, an operator who exports
// EVOLVE_BYPASS_SHIP_GATE=1 / EVOLVE_BYPASS_ROLE_GATE=1 in their dev shell (a
// common posture for manual git work) sees every Denies*/deny-path test fail:
// each guard's Decide() short-circuits to Allow at its first line, so the suite
// falsely reports a fail-OPEN security regression. CI passed because CI carries
// no such vars — a textbook "Local Hero" test smell that broke F.I.R.S.T.
// (Isolated / Repeatable).
//
// Tests that deliberately exercise the bypass path (e.g. TestShip_BypassEnvAllows)
// set the var with t.Setenv, which restores the now-absent prior state on cleanup,
// so this neutralization composes cleanly with them.
func TestMain(m *testing.M) {
	for _, v := range bypassEnvVars {
		os.Unsetenv(v)
	}
	os.Exit(m.Run())
}

// TestGuardsSuiteIsHermetic asserts the invariant TestMain establishes: no
// gate-bypass var leaks into the test process. It fails loudly if TestMain is
// removed or a new bypass var is added to the package without being listed
// here, preventing the false-failure regression from recurring silently.
func TestGuardsSuiteIsHermetic(t *testing.T) {
	for _, v := range bypassEnvVars {
		if got := os.Getenv(v); got != "" {
			t.Errorf("%s=%q leaked into the guards test process; TestMain must neutralize all bypass vars so deny-path tests reflect guard logic, not the ambient shell", v, got)
		}
	}
}
