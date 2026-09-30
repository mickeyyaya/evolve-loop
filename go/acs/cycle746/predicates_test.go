//go:build acs

package cycle746

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const evolveCmdPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

func TestC746_001_ReloadsMinLanesAtWaveBoundary(t *testing.T) {
	runGoTest(t, evolveCmdPkg, "TestFleetDispatch_ReloadsMinLanesAtWaveBoundary")
}

func TestC746_002_ReloadsCountAtWaveBoundary(t *testing.T) {
	runGoTest(t, evolveCmdPkg, "TestFleetDispatch_ReloadsCountAtWaveBoundary")
}

func TestC746_003_UnchangedPolicyByteIdenticalDispatch(t *testing.T) {
	runGoTest(t, evolveCmdPkg, "TestFleetDispatch_UnchangedPolicyByteIdenticalDispatch")
}

func TestC746_004_MalformedPolicyHoldsWidth(t *testing.T) {
	runGoTest(t, evolveCmdPkg, "TestFleetDispatch_MalformedPolicyAtWaveBoundaryHoldsWidth")
}
