//go:build acs

package cycle764

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	shipPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	rollbackPkg = "github.com/mickeyyaya/evolve-loop/go/internal/rollback"
)

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

func TestC764_001_DiscardNeverRemovesRunningExecutable(t *testing.T) {
	runGoTest(t, shipPkg, "TestDiscardBinaryChurn_NeverRemovesRunningExecutable")
}

func TestC764_002_ManualShipLeavesRunningBinaryDiscardsChurn(t *testing.T) {
	runGoTest(t, shipPkg, "TestManualShipSuccess_LeavesUntrackedGoBinEvolvePresent")
}

func TestC764_003_ShipAndRollbackSuitesNoRegression(t *testing.T) {
	for _, pkg := range []string{shipPkg, rollbackPkg} {
		stdout, stderr, code, err := acsassert.SubprocessOutput(
			"go", "test", "-race", "-count=1", pkg)
		if code != 0 || err != nil {
			t.Fatalf("full suite failed for %s (exit %d, err=%v)\nstdout:\n%s\nstderr:\n%s",
				pkg, code, err, stdout, stderr)
		}
	}
}
