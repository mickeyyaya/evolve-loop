//go:build acs

package cycle514

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d (package build failure or renamed tests)", got, min)
	}
}

func TestC514_001_AutoRepinsWhenProvenanceVerified(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery_AutoRepinsWhenProvenanceVerified", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot recovery does NOT auto-repin a provenance-verified SHA mismatch (exit=%d) — wire phaseintegrity.RepinShipSHA into defaultBootRecovery's mismatch branch\n%s", code, out)
	}
}

func TestC514_002_DeclinesRepinWhenProvenanceUnverified(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery_DeclinesRepinWhenProvenanceUnverified", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot recovery repins an UNVERIFIABLE binary (exit=%d) — anti-tamper broken; pass operatorAuthorized=false and refuse on provenance failure\n%s", code, out)
	}
}

func TestC514_003_NoExpectedSHAIsNoOpAndSkipsProvenance(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery_NoExpectedSHAIsNoOpAndSkipsProvenance", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot recovery does not short-circuit on an absent expected_ship_sha (exit=%d) — it must not invoke the provenance/git path when there is nothing pinned\n%s", code, out)
	}
}
