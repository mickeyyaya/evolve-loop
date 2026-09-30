//go:build acs

package cycle680

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modRoot = "github.com/mickeyyaya/evolve-loop/go/internal/"

func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+name+"$", modRoot+pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test %s reported no PASS for %s (renamed or not run?)\nstdout:\n%s", pkg, name, stdout)
	}
}

func TestC680_001_SelfCheckWriteFailureSurfacesWARN(t *testing.T) {
	runGoTest(t, "core", "TestWriteBuildSelfCheck_WriteFailureSurfaces")
}

func TestC680_002_SelfCheckMkdirFailureWARNsNoPanic(t *testing.T) {
	runGoTest(t, "core", "TestWriteBuildSelfCheckArtifact_MkdirAllFailure_WARNsAndDoesNotPanic")
}

func TestC680_003_SelfCheckHealthyWriteIsSilent(t *testing.T) {
	runGoTest(t, "core", "TestWriteBuildSelfCheck_HealthyWriteIsSilent")
}

func TestC680_004_SelfCheckHealthyRoundTrip(t *testing.T) {
	runGoTest(t, "core", "TestWriteBuildSelfCheckArtifact_HealthyRoundTrip_LargeAndUnicodeContent")
}

func TestC680_005_BreakerPersistFailureWARNs(t *testing.T) {
	runGoTest(t, "deliverable", "TestBreakerWriteFailureLogged")
}

func TestC680_006_BreakerTmpWriteFailureKeepsPriorState(t *testing.T) {
	runGoTest(t, "deliverable", "TestWriteBreaker_TmpWriteFailure_WARNsAndLeavesPriorStateUnchanged")
}

func TestC680_007_BreakerHealthyWriteIsSilent(t *testing.T) {
	runGoTest(t, "deliverable", "TestBreakerWriteSuccess_NoStderrNoise")
}
