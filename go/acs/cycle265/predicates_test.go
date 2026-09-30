//go:build acs

package cycle265

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

var (
	coverageRe = regexp.MustCompile(`coverage:\s+([0-9.]+)%\s+of statements`)
	passLineRe = regexp.MustCompile(`(?m)^--- PASS: (Test\w+)`)
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

func TestC265_002_AtLeastSevenInvariantTestsPass(t *testing.T) {
	out := runRoutingtestSuite(t)
	n := countTopLevelPass(out, "TestInvariant")
	if n < 7 {
		t.Errorf("RED: %d top-level TestInvariant* PASS, want >= 7 (one per invariant in invariantChecks)", n)
	}
}

func TestC265_003_AtLeastFiveBrickTestsPass(t *testing.T) {
	out := runRoutingtestSuite(t)
	n := countTopLevelPass(out, "TestBrick")
	if n < 5 {
		t.Errorf("RED: %d top-level TestBrick* PASS, want >= 5 (1 existing + >=4 new)", n)
	}
}

func TestC265_004_AtLeastOneEngineTestPass(t *testing.T) {
	out := runRoutingtestSuite(t)
	n := countTopLevelPass(out, "TestEngine")
	if n < 1 {
		t.Errorf("RED: %d top-level TestEngine* PASS, want >= 1 (RunAll/runPure pipeline)", n)
	}
}

func TestC265_005_DuplicatePhaseRejectedTestRunsAndPasses(t *testing.T) {
	out := runRoutingtestSuite(t)
	matched, _ := regexp.MatchString(`(?m)^--- PASS: TestInvariant_DuplicatePhaseRejected\b`, out)
	if !matched {
		t.Errorf("RED: TestInvariant_DuplicatePhaseRejected did not run+PASS (the negative duplicate-phase case must be exercised, not skipped)")
	}
}

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
