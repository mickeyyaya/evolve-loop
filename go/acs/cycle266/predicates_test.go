//go:build acs

package cycle266

import (
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	rtOnce sync.Once
	rtOut  string
)

func runRoutingtestSuite(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	rtOnce.Do(func() {
		goDir := filepath.Join(root, "go")
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir, "-count=1", "-cover", "-v",
			"./internal/routingtest/...")
		rtOut = stdout + "\n" + stderr
	})
	return rtOut
}

func invariantsGoPath(t *testing.T) string {
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "routingtest", "invariants.go")
}

func invariantsTestGoPath(t *testing.T) string {
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "routingtest", "invariants_test.go")
}

const newTest = "TestInvariant_NoDuplicatePhaseEnforcesUniqueness"

var (
	coverageRe   = regexp.MustCompile(`coverage:\s+([0-9.]+)%\s+of statements`)
	topPassRe    = regexp.MustCompile(`(?m)^--- PASS: (Test\w+)`)
	newSubPassRe = regexp.MustCompile(`(?m)^\s+--- PASS: ` + regexp.QuoteMeta(newTest) + `/(\S+)`)
	anyFailRe    = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

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

func topLevelPassed(out, name string) bool {
	for _, m := range topPassRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

func countTopLevelPass(out, prefix string) int {
	seen := map[string]bool{}
	for _, m := range topPassRe.FindAllStringSubmatch(out, -1) {
		if name := m[1]; len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			seen[name] = true
		}
	}
	return len(seen)
}

func newTestSubPasses(out string) map[string]bool {
	seen := map[string]bool{}
	for _, m := range newSubPassRe.FindAllStringSubmatch(out, -1) {
		seen[m[1]] = true
	}
	return seen
}

func TestC266_001_NoDuplicatePhaseInvariantRegistered(t *testing.T) {
	out := runRoutingtestSuite(t)
	if !topLevelPassed(out, newTest) {
		t.Errorf("RED: %s did not run+PASS — the `no-duplicate-phase` invariant is not registered/resolvable in invariantChecks", newTest)
	}
}

func TestC266_002_InvariantFiresOnDuplicatePhases(t *testing.T) {
	out := runRoutingtestSuite(t)
	if !topLevelPassed(out, newTest) {
		t.Errorf("RED: %s not PASSing — cannot confirm the invariant fires on duplicates", newTest)
	}
	if len(newTestSubPasses(out)) < 1 {
		t.Errorf("RED: no passing sub-case under %s — duplicate-detection path is not exercised", newTest)
	}
	src := invariantsGoPath(t)
	if !acsassert.FileContains(t, src, `"no-duplicate-phase"`) {
		t.Errorf("RED: invariants.go has no `no-duplicate-phase` invariantChecks entry")
	}
	if !acsassert.FileMatchesRegex(t, src, `t\.Errorf\([^)]*duplicate`) {
		t.Errorf("RED: invariants.go `no-duplicate-phase` has no t.Errorf firing site for a duplicate phase")
	}
}

func TestC266_003_DuplicatePhaseToleratedTestPasses(t *testing.T) {
	out := runRoutingtestSuite(t)
	if !topLevelPassed(out, "TestInvariant_DuplicatePhaseTolerated") {
		t.Errorf("RED: TestInvariant_DuplicatePhaseTolerated did not run+PASS (the tolerance/determinism scenario must survive the rename)")
	}
}

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

func TestC266_007_NegativeAndPositiveSubcases(t *testing.T) {
	out := runRoutingtestSuite(t)
	if subs := newTestSubPasses(out); len(subs) < 2 {
		t.Errorf("RED: %s has %d passing sub-cases, want >= 2 (one positive unique-plan, one negative duplicate-plan)", newTest, len(subs))
	}
}
