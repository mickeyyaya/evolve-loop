//go:build acs

package cycle778

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shipwindowPkg = "github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"

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

func TestC778_001_sibling_waits_instead_of_reaudit(t *testing.T) {
	runGoTest(t, shipwindowPkg, "TestShipWindowLease_SiblingWaitsInsteadOfReaudit")
}

func TestC778_002_held_lease_blocks_sibling(t *testing.T) {
	runGoTest(t, shipwindowPkg, "TestShipWindowLease_HeldLeaseBlocksSibling")
}

func TestC778_003_holder_death_recovered(t *testing.T) {
	runGoTest(t, shipwindowPkg, "TestShipWindowLease_HolderDeathRecovered")
}

func TestC778_004_fifo_fairness(t *testing.T) {
	runGoTest(t, shipwindowPkg, "TestShipWindowLease_FIFOFairness")
}

func TestC778_005_lease_path_contract(t *testing.T) {
	runGoTest(t, shipwindowPkg, "TestShipWindowLease_PathIn")
}
