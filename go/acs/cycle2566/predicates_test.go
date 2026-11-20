//go:build acs

package cycle2566

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func requireNamedTestPasses(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "test", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || !strings.Contains(stdout, "--- PASS: "+name+" ") {
		t.Errorf("%s in %s did not pass (exit %d):\n%s%s", name, pkg, code, acsassert.OutputTail(stdout, 15), acsassert.OutputTail(stderr, 5))
	}
}

func TestC2566_001_DrainReleaseEmitsInboxReleasedPerItem(t *testing.T) {
	requireNamedTestPasses(t, "./internal/inboxmover/lifecycle", "TestReleaseOne_EmitsInboxReleased")
}

func TestC2566_002_SweepWarnsOnSealedCycleStillHoldingAClaim(t *testing.T) {
	requireNamedTestPasses(t, "./internal/inboxmover", "TestStaleClaimSweep_SealedCycleStillInProcessingWarns")
}

func TestC2566_003_SystemHaltedFailureReleasesClaimsAndSignals(t *testing.T) {
	requireNamedTestPasses(t, "./internal/inboxmover", "TestApplyFailure_SystemHaltedCycleReleasesClaims")
}
