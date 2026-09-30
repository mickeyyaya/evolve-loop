//go:build acs

package cycle1130

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const auditPkg = "./internal/phases/audit/"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runGoTest(t *testing.T, pkg, pattern string, race bool) (ok bool, out string) {
	t.Helper()
	args := []string{"test", "-C", goDir(t), "-count=1"}
	if race {
		args = append(args, "-race")
	}
	args = append(args, "-run", "^("+pattern+")$", pkg)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, tail(out, 30))
	}
	return code == 0, out
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func TestC1130_001_ConflictAndGateFactsReachTheOperatorTogether(t *testing.T) {
	ok, out := runGoTest(t, auditPkg,
		"TestVerdictConflict_EGPSRed_NarrativeAndGateFactsArriveTogether|"+
			"TestVerdictConflict_ShipEligible_NarrativeAndGateFactsArriveTogether", false)
	if !ok {
		t.Errorf("RED: an EGPS override no longer hands the operator BOTH the auditor's declared "+
			"verdict and the gate's red_count/red-identity evidence at error severity — half the "+
			"forensic pair is missing from the dossier chain:\n%s", tail(out, 40))
	}
}

func TestC1130_002_GateEvidenceSurvivesWhereThereIsNoConflict(t *testing.T) {
	ok, out := runGoTest(t, auditPkg,
		"TestVerdictConflict_GateFactsSurviveWithoutAConflict|"+
			"TestVerdictConflict_CleanPassEmitsNeitherHalf", false)
	if !ok {
		t.Errorf("RED: the gate's evidence vanished on the coherent path, a conflict was fabricated "+
			"where auditor and gate AGREED, or the clean-PASS path leaked an error diagnostic — the "+
			"record must never replace or disturb what the gate already reported:\n%s", tail(out, 40))
	}
}

func TestC1130_003_ConflictFamilyIsRaceCleanAndRegressionFree(t *testing.T) {
	ok, out := runGoTest(t, auditPkg, "TestVerdictConflict_.*|TestEGPSRedDiagnostic_.*", true)
	if !ok {
		t.Errorf("RED: the verdict-conflict family (including the inherited cycle-1124/1127 suites "+
			"and the EGPS red-identity fingerprint contract) is not green under -race — either a "+
			"regression or a data race in the conflict path:\n%s", tail(out, 40))
	}
}
