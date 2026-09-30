//go:build acs

package cycle637

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC637_001_InsertedPhaseTransitionsViaPlan(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestRunCycleFromPhase_InsertedPhaseTransitionsViaPlan")
	if !ok {
		t.Errorf("resume does not rehydrate the transition kernel from routing-plan.json:\n%s", out)
	}
}

func TestC637_002_MissingPlanDegradesNotInvalidPhase(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestRunCycleFromPhase_MissingPlanDegradesNotInvalidPhase")
	if !ok {
		t.Errorf("resume does not degrade gracefully when routing-plan.json is missing:\n%s", out)
	}
}

func TestC637_003_TransitionFailureRecordsAbortReason(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestRunCycleFromPhase_TransitionFailureRecordsAbortReason")
	if !ok {
		t.Errorf("resume transition failure escapes the C1 chokepoint (no abort_reason recorded):\n%s", out)
	}
}
