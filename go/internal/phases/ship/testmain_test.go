package ship

import (
	"fmt"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
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
	code := m.Run()
	if err := gittest.RemoveTemplates(); err != nil {
		fmt.Fprintln(os.Stderr, "ship tests: remove git templates:", err)
	}
	os.Exit(code)
}

func TestShipSuiteIsHermetic(t *testing.T) {
	t.Parallel()
	for _, v := range shipControlFlowEnvVars {
		if got := os.Getenv(v); got != "" {
			t.Errorf("%s=%q leaked into the ship test process; TestMain must neutralize all control-flow vars so audit-binding tests aren't vacuously bypassed", v, got)
		}
	}
}
