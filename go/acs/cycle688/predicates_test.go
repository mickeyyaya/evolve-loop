//go:build acs

package cycle688

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const observerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/adapters/observer"

func runGoTest(t *testing.T, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", observerPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			observerPkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

func TestC688_001_TimeoutArmNeverClosesSink(t *testing.T) {
	runGoTest(t, "TestCoreAdapter_NoSinkCloseRaceOnTimeout")
}

func TestC688_002_DoneArmClosesExactlyOnce(t *testing.T) {
	runGoTest(t, "TestCoreAdapter_SinkClosedOnNormalDone")
}

func TestC688_003_NilCloserSafe(t *testing.T) {
	runGoTest(t, "TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe")
}

func TestC688_004_ObserverPackageRaceClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", observerPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			observerPkg, code, err, stdout, stderr)
	}
}

func TestC688_005_ObserverPackageVetClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", observerPkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			observerPkg, code, err, stdout, stderr)
	}
}
