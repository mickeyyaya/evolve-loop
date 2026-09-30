//go:build acs

package cycle639

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC639_001_HaltsPreScoutOnWithinVersionMismatch(t *testing.T) {
	ok, out := runGoTest(t, cmdEvolvePkg,
		"TestBootGate_HaltsOnWithinVersionSelfShaMismatch|TestRunLoop_HaltsPreScoutOnWithinVersionSelfShaMismatch")
	if !ok {
		t.Errorf("within-version ship-SHA mismatch does not HALT boot pre-scout with the operator recipe:\n%s", out)
	}
}

func TestC639_002_AcrossVersionStillAutoRepins(t *testing.T) {
	ok, out := runGoTest(t, cmdEvolvePkg, "TestBootGate_AcrossVersionMismatchStillAutoRepins")
	if !ok {
		t.Errorf("across-version mismatch no longer auto-repins at boot (existing behavior regressed):\n%s", out)
	}
}

func TestC639_003_MatchingSHABootsIntoScout(t *testing.T) {
	ok, out := runGoTest(t, cmdEvolvePkg, "TestBootGate_MatchingSHABootsIntoScout")
	if !ok {
		t.Errorf("a matched-SHA healthy tree does not boot cleanly into scout (spurious halt or action):\n%s", out)
	}
}
