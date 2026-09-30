//go:build acs

package cycle549

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var coveragePctRE = regexp.MustCompile(`coverage:\s+([0-9]+\.[0-9]+)% of statements`)

func runCoverage(t *testing.T, pkg, runFilter string) (pct float64, out string) {
	t.Helper()
	stdout, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-cover", "-run", runFilter, pkg)
	out = stdout + "\n" + stderr
	m := coveragePctRE.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no coverage percentage found in `go test -cover` output for %s (filter %q):\n%s", pkg, runFilter, out)
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("parse coverage percentage %q: %v", m[1], err)
	}
	return pct, out
}

func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "--- PASS") + strings.Count(out, "--- FAIL"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d (or the package failed to build — see output)", got, min)
	}
}

func TestC549_001_CmdutilCoverage_ClearsBar(t *testing.T) {
	pct, out := runCoverage(t, "github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil", ".")
	requireTestsRan(t, out, 3)
	if pct < 80.0 {
		t.Errorf("cmd/evolve/cmdutil coverage = %.1f%%, want >= 80.0%%\n%s", pct, out)
	}
}

func TestC549_002_CommitgateCoverage_ClearsBar(t *testing.T) {
	pct, out := runCoverage(t, "github.com/mickeyyaya/evolve-loop/go/internal/commitgate", ".")
	requireTestsRan(t, out, 15)
	if pct < 80.0 {
		t.Errorf("internal/commitgate coverage = %.1f%%, want >= 80.0%%\n%s", pct, out)
	}
}

func TestC549_003_WorktreeSwarmFunctions_FixtureCovered(t *testing.T) {
	runFilter := "TestRunWorktree|TestErrIsNotExist|TestRunSwarm|TestManifestPath|TestSwarmFixture"
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter,
		"github.com/mickeyyaya/evolve-loop/go/cmd/evolve")
	out := stdout + "\n" + stderr
	requireTestsRan(t, out, 20)
	if code != 0 {
		t.Errorf("worktree/swarm fixture tests failed (exit=%d)\n%s", code, out)
	}
	if strings.Contains(out, "--- FAIL") {
		t.Errorf("at least one worktree/swarm fixture test failed:\n%s", out)
	}
}
