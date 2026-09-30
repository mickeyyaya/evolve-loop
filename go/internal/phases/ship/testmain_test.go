package ship

import (
	"os"
	"testing"
)

// shipControlFlowEnvVars lists the env vars ship's control flow reads via
// os.Getenv; TestMain unsets them so the suite stays hermetic regardless of
// the operator's shell, and a test needing one sets it via t.Setenv.
var shipControlFlowEnvVars = []string{
	"EVOLVE_SHIP_AUTO_CONFIRM",
	"EVOLVE_SHIP_RELEASE_NOTES",
	"EVOLVE_BYPASS_PREFIX_GATE",
}

func TestMain(m *testing.M) {
	for _, v := range shipControlFlowEnvVars {
		os.Unsetenv(v)
	}
	os.Exit(m.Run())
}

func TestShipSuiteIsHermetic(t *testing.T) {
	for _, v := range shipControlFlowEnvVars {
		if got := os.Getenv(v); got != "" {
			t.Errorf("%s=%q leaked into the ship test process; TestMain must neutralize all control-flow vars so audit-binding tests aren't vacuously bypassed", v, got)
		}
	}
}
