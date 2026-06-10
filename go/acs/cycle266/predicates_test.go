//go:build acs

// Package cycle266 materializes the cycle-266 acceptance criteria for the
// `no-duplicate-phase-invariant` task: close the cycle-265 audit M1 WARN by
// turning the misnamed `TestInvariant_DuplicatePhaseRejected` (which only proved
// tolerance) into a real, behavior-grounded kernel-floor invariant.
//
//  1. add a `no-duplicate-phase` entry to `invariantChecks` in invariants.go
//     that calls t.Errorf when in.Plan.Entries repeats a Phase value;
//  2. rename the misleading test to `TestInvariant_DuplicatePhaseTolerated`
//     (its scenario genuinely asserts tolerance/determinism);
//  3. add `TestInvariant_NoDuplicatePhaseEnforcesUniqueness` with a positive
//     (unique plan → invariant silent) and a negative (duplicate plan → fires)
//     sub-case.
//
// These predicates are BEHAVIORAL (cycle-85 lesson): each RUNS the
// system-under-test — the `internal/routingtest` Go suite — as a subprocess and
// asserts on its real `go test -cover -v` output (top-level PASS lines, subtest
// PASS lines, coverage %, absence of FAIL). The load-bearing assertions exercise
// the actual invariant via the suite; any source-file checks are AUXILIARY
// anti-no-op guards, never the sole weight. The builder's job is production code
// (invariants.go) + the routingtest unit tests; these predicates gate it.
package cycle266

import (
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// --- one-shot runner: exercise the routingtest suite once, share the output ---

var (
	rtOnce sync.Once
	rtOut  string
)

// runRoutingtestSuite runs the full `internal/routingtest` suite with coverage
// and verbose output ONCE per predicate process and returns combined
// stdout+stderr. `-C <goDir>` makes the invocation cwd-independent (the audit
// lane may run from the worktree root or go/); `-count=1` defeats the test cache
// so PASS lines and coverage reflect the builder's just-written files.
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

// invariantsGoPath / invariantsTestGoPath resolve the two production files the
// builder edits (used only for AUXILIARY anti-no-op checks).
func invariantsGoPath(t *testing.T) string {
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "routingtest", "invariants.go")
}

func invariantsTestGoPath(t *testing.T) string {
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "routingtest", "invariants_test.go")
}

const newTest = "TestInvariant_NoDuplicatePhaseEnforcesUniqueness"

var (
	coverageRe = regexp.MustCompile(`coverage:\s+([0-9.]+)%\s+of statements`)
	// Top-level PASS lines are anchored at column 0; subtests are indented.
	topPassRe = regexp.MustCompile(`(?m)^--- PASS: (Test\w+)`)
	// Subtest PASS lines for the new test are indented and carry a `/sub` suffix.
	newSubPassRe = regexp.MustCompile(`(?m)^\s+--- PASS: ` + regexp.QuoteMeta(newTest) + `/(\S+)`)
	anyFailRe    = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
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

// topLevelPassed reports whether a column-0 `--- PASS: <name>` line is present.
func topLevelPassed(out, name string) bool {
	for _, m := range topPassRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

// countTopLevelPass returns the number of DISTINCT top-level test functions that
// PASSed whose name begins with prefix (subtests excluded by the column anchor).
func countTopLevelPass(out, prefix string) int {
	seen := map[string]bool{}
	for _, m := range topPassRe.FindAllStringSubmatch(out, -1) {
		if name := m[1]; len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			seen[name] = true
		}
	}
	return len(seen)
}

// newTestSubPasses returns the distinct passing subtest names under the new test.
func newTestSubPasses(out string) map[string]bool {
	seen := map[string]bool{}
	for _, m := range newSubPassRe.FindAllStringSubmatch(out, -1) {
		seen[m[1]] = true
	}
	return seen
}

// --- C1: `no-duplicate-phase` is a registered invariant in invariantChecks ---

// The new test's POSITIVE scenario declares ExpectInvariants("no-duplicate-phase").
// If the key is absent from invariantChecks, assertInvariants() calls
// t.Fatalf("unknown invariant %q") and the test FAILS — so a top-level PASS of
// the new test behaviorally proves the invariant resolves (is registered). A
// magic string in a doc cannot produce this PASS line.
func TestC266_001_NoDuplicatePhaseInvariantRegistered(t *testing.T) {
	out := runRoutingtestSuite(t)
	if !topLevelPassed(out, newTest) {
		t.Errorf("RED: %s did not run+PASS — the `no-duplicate-phase` invariant is not registered/resolvable in invariantChecks", newTest)
	}
}

// --- C2: the invariant FIRES t.Errorf on duplicate plan phases ---

// Load-bearing (behavioral): the new test PASSES with a negative sub-case that
// exercises duplicate detection — if the firing logic were a no-op, that
// sub-case (which asserts duplicates ARE detected) would FAIL and there would be
// no top-level PASS. AUXILIARY anti-no-op: invariants.go carries the
// `no-duplicate-phase` key AND a `t.Errorf(...duplicate phase...)` firing site,
// so the registered invariant has real error-emitting logic, not a silent stub.
func TestC266_002_InvariantFiresOnDuplicatePhases(t *testing.T) {
	out := runRoutingtestSuite(t)
	if !topLevelPassed(out, newTest) {
		t.Errorf("RED: %s not PASSing — cannot confirm the invariant fires on duplicates", newTest)
	}
	if len(newTestSubPasses(out)) < 1 {
		t.Errorf("RED: no passing sub-case under %s — duplicate-detection path is not exercised", newTest)
	}
	// AUXILIARY: the production invariant has firing logic referencing duplicates.
	src := invariantsGoPath(t)
	if !acsassert.FileContains(t, src, `"no-duplicate-phase"`) {
		t.Errorf("RED: invariants.go has no `no-duplicate-phase` invariantChecks entry")
	}
	if !acsassert.FileMatchesRegex(t, src, `t\.Errorf\([^)]*duplicate`) {
		t.Errorf("RED: invariants.go `no-duplicate-phase` has no t.Errorf firing site for a duplicate phase")
	}
}

// --- C3: the renamed TestInvariant_DuplicatePhaseTolerated runs and PASSES ---

func TestC266_003_DuplicatePhaseToleratedTestPasses(t *testing.T) {
	out := runRoutingtestSuite(t)
	if !topLevelPassed(out, "TestInvariant_DuplicatePhaseTolerated") {
		t.Errorf("RED: TestInvariant_DuplicatePhaseTolerated did not run+PASS (the tolerance/determinism scenario must survive the rename)")
	}
}

// --- C4: the misleading TestInvariant_DuplicatePhaseRejected name is removed ---

// A removal is verified by absence on BOTH axes: the old name must not appear as
// a top-level PASS in the live suite output, AND it must be gone from the source.
// (A bare `-run` of a nonexistent test exits 0, so the source check is the
// load-bearing half here — legitimate for a deletion criterion.)
func TestC266_004_OldRejectedTestNameRemoved(t *testing.T) {
	const old = "TestInvariant_DuplicatePhaseRejected"
	out := runRoutingtestSuite(t)
	if topLevelPassed(out, old) {
		t.Errorf("RED: %s still runs — it must be renamed to ...Tolerated", old)
	}
	if acsassert.FileContainsAny(invariantsTestGoPath(t), old) {
		t.Errorf("RED: invariants_test.go still references %s — rename incomplete", old)
	}
}

// --- C5: routingtest package coverage stays >= 80% (regression guard) ---

// NOTE: coverage is 80.6% at the cycle-266 baseline, so this criterion is a
// MAINTAIN/regression guard and may report pre-existing GREEN at RED time.
func TestC266_005_CoverageAtLeast80(t *testing.T) {
	out := runRoutingtestSuite(t)
	cov := parseCoverage(out)
	if cov < 0 {
		t.Fatalf("RED: no `coverage: N%% of statements` line — suite did not build/run.\n%s", out)
	}
	if cov < 80.0 {
		t.Errorf("RED: routingtest coverage = %.1f%%, want >= 80.0%% (baseline 80.6%%)", cov)
	}
}

// --- C6: no regression — suite green + invariant-test count grows to >= 10 ---

// "No regression" means (a) zero FAIL lines anywhere, (b) the framework keystone
// TestSignalSpec_DualRenderingAgree still PASSes, and (c) the count of top-level
// TestInvariant_* functions reaches 10 (the 9 baseline, with Rejected→Tolerated
// net-zero, PLUS the new EnforcesUniqueness). A drop below 10 means a prior
// invariant test was lost or the new one is missing.
func TestC266_006_NoRegression(t *testing.T) {
	out := runRoutingtestSuite(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: routingtest suite has a FAIL line — no regressions allowed.\n%s", out)
	}
	if !topLevelPassed(out, "TestSignalSpec_DualRenderingAgree") {
		t.Errorf("RED/REGRESSION: framework keystone TestSignalSpec_DualRenderingAgree is not PASSing")
	}
	if n := countTopLevelPass(out, "TestInvariant"); n < 10 {
		t.Errorf("RED: %d top-level TestInvariant_* PASS, want >= 10 (9 baseline + new EnforcesUniqueness)", n)
	}
}

// --- C7: the new test has BOTH a positive and a negative sub-case (>= 2) ---

// Adversarial diversity (skills/adversarial-testing §6): a positive-only test is
// gameable by a no-op invariant. Requiring >= 2 passing sub-cases under the new
// test forces the negative (duplicate → fires) path to be exercised alongside
// the positive (unique → silent) one.
func TestC266_007_NegativeAndPositiveSubcases(t *testing.T) {
	out := runRoutingtestSuite(t)
	if subs := newTestSubPasses(out); len(subs) < 2 {
		t.Errorf("RED: %s has %d passing sub-cases, want >= 2 (one positive unique-plan, one negative duplicate-plan)", newTest, len(subs))
	}
}
