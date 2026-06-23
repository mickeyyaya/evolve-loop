//go:build acs

// Package cycle265 materializes the cycle-265 acceptance criteria for the
// `routingtest-coverage` task: push `internal/routingtest` package coverage from
// 23% to >=70% by exercising the 8 kernel-floor invariants, the
// RunAll/runPure/buildConfig engine pipeline, and additional Brick functions.
//
// These predicates are BEHAVIORAL (cycle-85 lesson): each one RUNS the
// system-under-test — here, the `routingtest` Go suite — as a subprocess and
// asserts on its real `go test -cover -v` output (coverage %, top-level PASS
// counts, absence of FAIL). None greps a source file. If the builder deletes a
// test, coverage and the PASS counts drop and these predicates fail. The
// builder's job is test files only; production code is out of scope.
package cycle265

import (
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// --- one-shot runner: exercise the routingtest suite once, share the output ---

var (
	rtOnce sync.Once
	rtOut  string
)

// runRoutingtestSuite runs the full `internal/routingtest` suite with coverage
// and verbose output ONCE per predicate process and returns the combined
// stdout+stderr. `-C <goDir>` makes the invocation cwd-independent (the audit
// lane may run from the worktree root or go/); `-count=1` defeats the test cache
// so coverage is recomputed against the builder's just-written files.
func runRoutingtestSuite(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t) // t.Skip when not in a git work tree
	rtOnce.Do(func() {
		goDir := filepath.Join(root, "go")
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir, "-count=1", "-cover", "-v",
			"./internal/routingtest/...")
		rtOut = stdout + "\n" + stderr
	})
	return rtOut
}

var (
	coverageRe = regexp.MustCompile(`coverage:\s+([0-9.]+)%\s+of statements`)
	// Top-level PASS lines are anchored at column 0 (`--- PASS: TestX (...)`);
	// subtests are indented, so `(?m)^--- PASS:` never matches a subtest line.
	passLineRe = regexp.MustCompile(`(?m)^--- PASS: (Test\w+)`)
)

// parseCoverage extracts the reported statement coverage percentage, or -1 if
// the suite produced no coverage line (compile failure / no package).
func parseCoverage(out string) float64 {
	m := coverageRe.FindStringSubmatch(out)
	if m == nil {
		return -1
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return -1
	}
	return v
}

// countTopLevelPass returns the number of DISTINCT top-level test functions that
// PASSed whose name begins with prefix. De-duping by name guards against any
// accidental double emission; the column-0 anchor already excludes subtests.
func countTopLevelPass(out, prefix string) int {
	seen := map[string]bool{}
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		name := m[1]
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			seen[name] = true
		}
	}
	return len(seen)
}

// --- C1: coverage >= 70% (the headline criterion) ---

func TestC265_001_RoutingtestCoverageAtLeast70(t *testing.T) {
	out := runRoutingtestSuite(t)
	cov := parseCoverage(out)
	if cov < 0 {
		t.Fatalf("RED: no `coverage: N%% of statements` line — suite did not build/run.\n%s", out)
	}
	if cov < 70.0 {
		t.Errorf("RED: routingtest coverage = %.1f%%, want >= 70.0%% (baseline 23%%)", cov)
	}
}

// --- C2: >= 7 TestInvariant_* top-level tests PASS (one per invariantChecks key) ---

func TestC265_002_AtLeastSevenInvariantTestsPass(t *testing.T) {
	out := runRoutingtestSuite(t)
	n := countTopLevelPass(out, "TestInvariant")
	if n < 7 {
		t.Errorf("RED: %d top-level TestInvariant* PASS, want >= 7 (one per invariant in invariantChecks)", n)
	}
}

// --- C3: >= 5 TestBrick* top-level tests PASS ---

func TestC265_003_AtLeastFiveBrickTestsPass(t *testing.T) {
	out := runRoutingtestSuite(t)
	n := countTopLevelPass(out, "TestBrick")
	if n < 5 {
		t.Errorf("RED: %d top-level TestBrick* PASS, want >= 5 (1 existing + >=4 new)", n)
	}
}

// --- C4: >= 1 TestEngine* top-level test PASS (exercises RunAll path) ---

func TestC265_004_AtLeastOneEngineTestPass(t *testing.T) {
	out := runRoutingtestSuite(t)
	n := countTopLevelPass(out, "TestEngine")
	if n < 1 {
		t.Errorf("RED: %d top-level TestEngine* PASS, want >= 1 (RunAll/runPure pipeline)", n)
	}
}

// --- C5 (negative/adversarial): the duplicate-phase rejection test must RUN and PASS ---

// A bare `go test -run TestInvariant_DuplicatePhaseRejected` exits 0 even when NO
// such test exists ("no tests to run"). Asserting the exact `--- PASS:` line
// proves the test actually ran — this is the anti-no-op guard for C5.
func TestC265_005_DuplicatePhaseRejectedTestRunsAndPasses(t *testing.T) {
	out := runRoutingtestSuite(t)
	matched, _ := regexp.MatchString(`(?m)^--- PASS: TestInvariant_DuplicatePhaseRejected\b`, out)
	if !matched {
		t.Errorf("RED: TestInvariant_DuplicatePhaseRejected did not run+PASS (the negative duplicate-phase case must be exercised, not skipped)")
	}
}

// --- C6 (regression guard): the framework keystone stays GREEN; no FAIL anywhere ---

// This criterion is expected to be pre-existing GREEN: it asserts the new tests
// do not break the dual-rendering keystone and that the suite has zero failures.
func TestC265_006_NoRegression(t *testing.T) {
	out := runRoutingtestSuite(t)
	keystone, _ := regexp.MatchString(`(?m)^--- PASS: TestSignalSpec_DualRenderingAgree\b`, out)
	if !keystone {
		t.Errorf("RED/REGRESSION: TestSignalSpec_DualRenderingAgree is not PASSing — framework keystone broke")
	}
	if failed, _ := regexp.MatchString(`(?m)^(\s*)--- FAIL:`, out); failed {
		t.Errorf("RED/REGRESSION: routingtest suite has a FAIL line — no regressions allowed.\n%s", out)
	}
}
