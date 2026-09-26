package guards

import (
	"os"
	"testing"
)

// bypassEnvVars are gate-bypass variables an operator shell may still export. The guards take bypass
// from their constructors and read none of them; TestMain unsets them so no test depends on the shell.
var bypassEnvVars = []string{
	"EVOLVE_BYPASS_ROLE_GATE",
	"EVOLVE_BYPASS_SHIP_GATE",
	"EVOLVE_BYPASS_PHASE_GATE",
}

func TestMain(m *testing.M) {
	for _, v := range bypassEnvVars {
		os.Unsetenv(v)
	}
	os.Exit(m.Run())
}

func TestGuardsSuiteIsHermetic(t *testing.T) {
	for _, v := range bypassEnvVars {
		if got := os.Getenv(v); got != "" {
			t.Errorf("%s=%q leaked into the guards test process; TestMain must neutralize all bypass vars so deny-path tests reflect guard logic, not the ambient shell", v, got)
		}
	}
}
