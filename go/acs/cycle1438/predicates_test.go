//go:build acs

// Package cycle1438 materialises the cycle-1438 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	salvage-backtick-regression-guard → land the missing regression coverage that
//	formally closes the cycle-1406/1407 `isQuotedEcho` backtick-adjacency defect.
//
// What this cycle is (and is NOT). Scout verified against the live tree — not the
// retro text — that the buggy `isQuotedEcho`/`insideStringLiteral` heuristic is
// GONE: `ClassifyBadVerdict` (go/internal/deliverable/salvage_instrument.go) was
// rewritten wholesale into a 4-shape precedence classifier that does no
// adjacency detection at all. So the production fix is already present and the
// acceptance bar is NOT "change the classifier" — it is "pin the fixed behaviour
// with a durable, committed test so it cannot regress silently". Predicate 004
// is the anti-scope-creep crux: it asserts the classifier's observable contract
// is UNCHANGED, so a Builder who 'improves' already-correct production code
// fails this cycle.
//
// Predicate strategy — every predicate exercises the system under test (a real
// call into ClassifyBadVerdict, or a real `go test` subprocess whose PASS lines
// are read), never a source-grep of production code as the load-bearing check
// (the cycle-85 degenerate-predicate ban):
//
//   - 001 shells the named regression test and requires a real `--- PASS:` line
//     for it, so "no tests to run" (exit 0, test absent) cannot green it. RED
//     today: the test does not exist.
//   - 002 does the same for the symbol-reintroduction guard test. RED today.
//   - 003 is the behavioural property itself, asserted by DIRECT CALL: a stray
//     unmatched backtick must not perturb classification, proven by a paired
//     control (backtick-bearing input vs its backtick-free twin) across all four
//     classifier shapes. Pre-existing GREEN — it pins the already-landed fix.
//   - 004 is the golden contract of the classifier over the four canonical
//     shapes (exact Pattern + Recoverable). Pre-existing GREEN; it red-fails any
//     production-code edit that alters observable classification, and its
//     negative axis (a genuinely-absent verdict must stay NOT recoverable)
//     kills a stub that blanket-returns Recoverable.
//   - 005 is the no-regression floor: the whole named package still greens.
//
// Root resolution: acsassert.RepoRoot(t) is the worktree, where Builder writes
// the new test — the deliverable is a worktree source change, so its absence is
// a FAILURE, not a skip. Subprocesses set cmd.Dir explicitly (never inherit the
// process cwd, which differs between main tree, worktree and each fleet lane).
package cycle1438

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// targetPkg is the ONE named package these predicates compile and test. Never a
// `./...` sweep: whole-repo staleness is the regression suite's job, and a
// multi-package invocation under fleet load is the flaky-predicate shape that
// false-redded cycles 1173/1175/1178.
const targetPkg = "./internal/deliverable"

// regressionTestName is the durable regression test the Builder must land: it
// feeds ClassifyBadVerdict a sentinel preceded by a stray, never-closed backtick
// and asserts the classification is identical to the backtick-free twin.
const regressionTestName = "TestClassifyBadVerdict_UnmatchedBacktickDoesNotMisclassify"

// guardTestName is the symbol-reintroduction guard. The cycle-1406/1407 defect
// lived in `isQuotedEcho` (helped by `insideStringLiteral`); both are gone, and
// this test is the tripwire that fires if adjacency-as-proof logic is ever
// reintroduced into the package. The name is PINNED here (scout left "same test
// or a separate func" open) so the acceptance criterion is deterministic.
const guardTestName = "TestNoQuotedEchoRegression"

// testFileRel is where both tests must land — the package's existing test file,
// not a new one, per the task's single targetFiles entry.
const testFileRel = "go/internal/deliverable/salvage_instrument_test.go"

// strayBacktickPreamble is the poison the historical bug choked on: a lone,
// never-closed backtick sitting in prose ahead of the report's own verdict
// shape. Under the old isQuotedEcho heuristic this "proved" the verdict was a
// quoted echo, yielding a false "genuinely absent, not recoverable".
const strayBacktickPreamble = "The auditor noted a stray ` tick in the transcript and moved on.\n\n"

// goTest runs `go test` for ONE named package from the worktree's go/ module
// root, with an explicit context deadline and cmd.Dir. Returns combined output
// and the exit code.
func goTest(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	full := append([]string{"test", "-count=1"}, args...)
	cmd := exec.CommandContext(ctx, "go", full...)
	cmd.Dir = filepath.Join(root, "go")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		code = cmd.ProcessState.ExitCode()
		if code < 0 {
			code = 1
		}
	}
	return string(out), code
}

// requireNamedTestPasses is the shared shape of predicates 001/002: run exactly
// one named test verbosely and demand its own `--- PASS:` line. Exit code alone
// is NOT sufficient evidence — `go test -run` over a pattern that matches
// nothing exits 0 with "no tests to run", which would green an absent test.
func requireNamedTestPasses(t *testing.T, root, name string) {
	t.Helper()
	out, code := goTest(t, root, "-v", "-run", "^"+name+"$", targetPkg)
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Errorf("RED: %s did not run-and-pass in %s (exit=%d).\n"+
			"A `--- PASS: %s` line is required — exit 0 with \"no tests to run\" means the test is absent.\n"+
			"---- go test output ----\n%s", name, targetPkg, code, name, out)
		return
	}
	if code != 0 {
		t.Errorf("RED: %s passed but the package run exited %d:\n%s", name, code, out)
	}
}

// TestC1438_001_RegressionTestRunsAndPasses is the primary acceptance criterion:
// the durable backtick regression test exists, executes, and passes.
func TestC1438_001_RegressionTestRunsAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, testFileRel)) {
		t.Fatalf("RED: %s missing on disk", testFileRel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", testFileRel); code != 0 {
		t.Errorf("RED: %s is untracked — an untracked test is dropped at ship and pins nothing", testFileRel)
	}
	requireNamedTestPasses(t, root, regressionTestName)
}

// TestC1438_002_SymbolReintroductionGuardRunsAndPasses pins the tripwire that
// fires if the removed adjacency heuristic ever comes back.
func TestC1438_002_SymbolReintroductionGuardRunsAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	requireNamedTestPasses(t, root, guardTestName)
}

// backtickCases are the four classifier shapes, each as a backtick-free control.
// Predicate 003 runs every one of them twice: once as written, once with the
// stray-backtick preamble prepended. Classification must be byte-identical.
func backtickCases() []struct {
	name    string
	content string
} {
	return []struct {
		name    string
		content string
	}{
		{
			// The historical crux: a sentinel whose payload is malformed by a
			// trailing comma. This is the shape the old bug misread.
			name:    "sentinel-trailing-comma",
			content: "# Audit Report\n\n<!-- evolve-verdict: {\"verdict\":\"FAIL\",} -->\n",
		},
		{
			name:    "fenced-json",
			content: "# Audit Report\n\n```json\n{\"verdict\":\"PASS\"}\n```\n",
		},
		{
			name:    "displaced-line",
			content: "# Audit Report\n\nThe verdict object is {\"verdict\":\"PASS\"} inline in prose.\n",
		},
		{
			// The negative axis: nothing recoverable anywhere. A classifier that
			// blanket-claims recoverability fails here.
			name:    "none",
			content: "# Audit Report\n\nProse only. No verdict payload of any kind was emitted.\n",
		},
	}
}

// TestC1438_003_UnmatchedBacktickDoesNotPerturbClassification is the behavioural
// property this cycle exists to pin, asserted by direct call with a paired
// control. Proving "same as the backtick-free twin" is strictly stronger than
// asserting today's literal output: it shows the backtick made NO difference.
func TestC1438_003_UnmatchedBacktickDoesNotPerturbClassification(t *testing.T) {
	for _, tc := range backtickCases() {
		control := deliverable.ClassifyBadVerdict(tc.content)
		poisoned := deliverable.ClassifyBadVerdict(strayBacktickPreamble + tc.content)
		if poisoned.Recoverable != control.Recoverable || poisoned.Pattern != control.Pattern {
			t.Errorf("RED[%s]: a stray unmatched backtick perturbed classification — "+
				"control{recoverable=%v pattern=%q} vs poisoned{recoverable=%v pattern=%q}",
				tc.name, control.Recoverable, control.Pattern, poisoned.Recoverable, poisoned.Pattern)
		}
		if poisoned.Reason == "" {
			t.Errorf("RED[%s]: classification carries an empty Reason — a silent classification is not observability", tc.name)
		}
	}

	// Edge axis: a bare stray backtick with no report body at all, and an empty
	// deliverable. Neither may be claimed as recoverable.
	for _, content := range []string{"", "`", strayBacktickPreamble} {
		if got := deliverable.ClassifyBadVerdict(content); got.Recoverable {
			t.Errorf("RED: content %q classified Recoverable=true pattern=%q — a verdict-free deliverable is not recoverable",
				content, got.Pattern)
		}
	}
}

// TestC1438_004_ClassifierContractUnchanged is the anti-scope-creep crux. The
// production classifier is already correct; this cycle must not edit it. The
// golden table below is its observable contract as verified live by scout — any
// production edit that changes classification red-fails here.
func TestC1438_004_ClassifierContractUnchanged(t *testing.T) {
	want := map[string]struct {
		recoverable bool
		pattern     deliverable.SalvagePattern
	}{
		"sentinel-trailing-comma": {true, deliverable.SalvagePatternTrailingComma},
		"fenced-json":             {true, deliverable.SalvagePatternFencedJSON},
		"displaced-line":          {true, deliverable.SalvagePatternDisplaced},
		"none":                    {false, deliverable.SalvagePatternNone},
	}
	for _, tc := range backtickCases() {
		exp, ok := want[tc.name]
		if !ok {
			t.Fatalf("golden table is missing case %q", tc.name)
		}
		got := deliverable.ClassifyBadVerdict(tc.content)
		if got.Recoverable != exp.recoverable || got.Pattern != exp.pattern {
			t.Errorf("RED[%s]: classifier contract changed — want{recoverable=%v pattern=%q} got{recoverable=%v pattern=%q}. "+
				"This cycle must NOT edit salvage_instrument.go; the production fix is already correct.",
				tc.name, exp.recoverable, exp.pattern, got.Recoverable, got.Pattern)
		}
	}

	// Auxiliary (NOT load-bearing): the removed heuristic must stay removed.
	root := acsassert.RepoRoot(t)
	prod := filepath.Join(root, "go", "internal", "deliverable", "salvage_instrument.go")
	for _, sym := range []string{"isQuotedEcho", "insideStringLiteral"} {
		if !acsassert.FileNotContains(t, prod, sym) {
			t.Errorf("RED: %s reintroduced into salvage_instrument.go — that is the cycle-1406/1407 defect returning", sym)
		}
	}
}

// TestC1438_005_PackageSuiteGreen is the no-regression floor: adding the new
// tests must not break the package. ONE named package, never a `./...` sweep.
func TestC1438_005_PackageSuiteGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, code := goTest(t, root, targetPkg)
	if code != 0 {
		t.Errorf("RED: `go test %s` exited %d — regression in the touched package:\n%s", targetPkg, code, out)
	}
}
