//go:build acs

// Package cycle1073 materialises the cycle-1073 acceptance criteria for the
// single fleet-scoped task pinned to this lane:
//
//	tdd-topn-scope-gate → convert tddScopeGate.check's case-2 (non-empty
//	committed ## top_n + an authored slug with zero overlap) from a fatal
//	block to the advisory label-drift pattern topNBindingGate already uses
//	(commit cbd088a1, #348), while KEEPING case-1 (empty top_n + a non-empty
//	authored set) fatal.
//
// Why this cycle exists. #348 converted the build->audit gate after two
// recorded false rejections (cycles 916, 1012) discarded CORRECT work whose
// report merely labelled the committed task differently — two LLM-authored
// strings compared for exact equality. The sibling tddScopeGate, which guards
// the triage->TDD transition one phase EARLIER, was not touched by that fix and
// still hard-blocks on the identical comparison. Case 1 is genuinely different
// (there is no committed item the authored files could be a relabelling of) and
// must stay fatal — so a blanket "never block" rewrite is a regression, not a
// fix, and predicate 002 exists to catch exactly that overcorrection.
//
// Predicate strategy — every predicate EXERCISES the gate by running the real
// unit tests that drive tddScopeGate.check directly (white-box, same package),
// never a source-grep of gate.go (the cycle-85 degenerate-predicate ban): a
// magic string added to the source cannot green any of these, only a change to
// the value check() actually returns can.
//
//   - 001 (crux, RED now) — label drift returns block=false with a reason that
//     names the drift and both slug sets.
//   - 002 (negative axis, GREEN now — anti-overcorrection) — an authored set
//     under an EMPTY top_n still returns block=true.
//   - 003 (edge/fail-open axis + no-regression) — the whole topngate package
//     suite is green, covering the six ambiguity cases (missing reports, empty
//     authored set, unparseable claim) and the untouched build-side gate.
//   - 004 — `go vet` is clean on the touched package (the AC pins the exported
//     signature of check(): only the returned bool and string content change).
package cycle1073

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const topngatePkg = "github.com/mickeyyaya/evolve-loop/go/internal/topngate"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure or
// an assertion failure in the target package surfaces as a non-zero exit — the
// intended RED signal before Builder changes the gate. code < 0 is a genuine
// launch failure (binary missing / killed), never a test verdict, so it is a
// hard error rather than a silent RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	args := []string{"test", "-count=1", pkg}
	if pattern != "" {
		args = []string{"test", "-run", "^(" + pattern + ")$", "-count=1", pkg}
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC1073_001_LabelDriftAtTriageToTDDIsAdvisory is the cycle crux. Drives the
// REAL tddScopeGate.check with a workspace whose triage-report.md commits one
// slug and whose test-report.md claims another: the gate must return
// block=false and a populated "label drift" reason naming both slug sets. RED
// now (gate.go:125-126 returns block=true); GREEN once case 2 mirrors
// topNBindingGate's advisory return.
func TestC1073_001_LabelDriftAtTriageToTDDIsAdvisory(t *testing.T) {
	ok, out := runGoTest(t, topngatePkg, "TestTDDScopeGate_LabelDriftIsAdvisory")
	if !ok {
		t.Errorf("tddScopeGate still HARD-BLOCKS on a differently-labelled-but-committed task — the same false-rejection defect #348 closed for the build gate, one phase earlier:\n%s", out)
	}
}

// TestC1073_002_EmptyTopNAuthoringStaysFatal is the negative / anti-gaming
// axis: the fix must NOT be a blanket block=false. An authored set under an
// EMPTY committed top_n is orphan authoring, not a labelling dispute, and must
// still abort the cycle. This predicate is GREEN today and must STAY green —
// it fails precisely on the plausible overcorrection that would green 001.
func TestC1073_002_EmptyTopNAuthoringStaysFatal(t *testing.T) {
	ok, out := runGoTest(t, topngatePkg, "TestTDDScopeGate_EmptyTopNStillBlocks")
	if !ok {
		t.Errorf("the advisory conversion overreached: orphan TDD authoring under an EMPTY ## top_n must stay a hard block (gate.go's documented case-1 carve-out):\n%s", out)
	}
}

// TestC1073_003_TopngatePackageSuiteGreen covers the edge/fail-open axis and
// guards against collateral damage: the whole topngate package must pass,
// including the six tddScopeGate ambiguity subtests (missing triage-report,
// missing test-report, empty authored set, unparseable claim, in-lane pass,
// multi-member top_n), the tdd-phase-scoping test, and the UNTOUCHED
// topNBindingGate + reviewer suites.
func TestC1073_003_TopngatePackageSuiteGreen(t *testing.T) {
	ok, out := runGoTest(t, topngatePkg, "")
	if !ok {
		t.Errorf("the topngate package suite is not green — the gate change broke a fail-open path or the sibling build gate:\n%s", out)
	}
}

// TestC1073_004_TouchedPackageVetsClean pins the AC that only the returned bool
// and reason string change: `go vet` on the touched package must be silent (CI's
// vet+fmt step would otherwise FAIL the cycle).
func TestC1073_004_TouchedPackageVetsClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", topngatePkg)
	out := strings.TrimSpace(stdout + stderr)
	if code < 0 {
		t.Fatalf("go vet failed to launch: code=%d err=%v\n%s", code, err, out)
	}
	if code != 0 || out != "" {
		t.Errorf("go vet %s must be clean; exit=%d output:\n%s", topngatePkg, code, out)
	}
}
