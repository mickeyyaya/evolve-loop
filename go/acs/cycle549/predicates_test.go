//go:build acs

// Package cycle549 materialises the cycle-549 acceptance criteria for this
// fleet lane's (cycle-21f9f7ae-549, fleet_scope
// cli-command-layer-test-coverage-worktree-swarm) sole `## top_n` task per
// triage-report.md:
//
//	cli-command-layer-test-coverage — raise cmd/evolve (esp.
//	runWorktreeCreate/List/Cleanup, runSwarmReap), cmd/evolve/cmdutil, and
//	internal/commitgate (incl. non-Go lane fixtures/removal) to >=80% tagged
//	coverage with fixture-based success+error-path tests.
//
// NOTE: the cycle-549 scout-report.md (this same workspace) ALSO proposed two
// OTHER tasks (memo-activation-overlay-layering,
// unroutable-phase-fail-loudly) as its own "## Selected Tasks" — but
// triage-report.md explicitly scoped THIS lane to
// cli-command-layer-test-coverage-worktree-swarm only and left those two
// tasks for a sibling lane to triage/build independently (see
// triage-report.md's "Fleet-lane scope" section). Per the AC-Materialization
// Contract (R9.3: "predicates bind ONLY to triage-committed work"), this
// package predicates ONLY the triage-committed item above.
//
// Predicate strategy: this is a COVERAGE-COMPLETION task, not a new-behavior
// task — the production code under test (cmd_worktree.go, cmd_swarm.go,
// cmdutil.go, lanes.go) already works correctly; the gap was test coverage,
// not functionality. So these predicates are BEHAVIORAL in a different sense
// than a RED/GREEN feature: each drives `go test -cover` as a subprocess over
// the real package and asserts (a) the new fixture-based tests actually ran
// (non-degenerate — closes the cycle-85 "no tests to run" trap) and (b) the
// reported coverage percentage clears the committed bar. This is exactly the
// "pre-existing GREEN" disposition test-report.md documents (Step 4's RED
// verification rules) — never a source grep.
//
// In-package tests authored by the TDD engineer this cycle:
//
//	cmd/evolve/cmd_worktree_test.go
//	cmd/evolve/cmd_swarm_test.go
//	cmd/evolve/cmdutil/filterenv_promptsloader_test.go
//	internal/commitgate/lanes_test.go
//
// The Builder's role for this task (if any remains) is limited to fixing any
// GENUINE bug these tests surface; per Step 4 they are pre-existing GREEN
// (the production code was already correct), so no Builder code change is
// expected — Builder should confirm the four coverage bars below still hold
// after any concurrent change and must not modify these test files.
package cycle549

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var coveragePctRE = regexp.MustCompile(`coverage:\s+([0-9]+\.[0-9]+)% of statements`)

// runCoverage runs `go test <pkg> -cover -run <runFilter>` and returns the
// reported coverage percentage plus the combined output (for diagnostics on
// failure). runFilter narrows to the new fixture-based tests this cycle
// added, so a regression in an UNRELATED pre-existing test in the same
// package cannot mask a real drop in the targeted functions' coverage.
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

// TestC549_001_CmdutilCoverage_ClearsBar: cmd/evolve/cmdutil (FilterEvolveEnv,
// NewPromptsLoader — both 0.0% before this cycle) is committed to >=80%
// package coverage. Drives filterenv_promptsloader_test.go +
// cmdutil_test.go's existing TestHasHelp.
func TestC549_001_CmdutilCoverage_ClearsBar(t *testing.T) {
	pct, out := runCoverage(t, "github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil", ".")
	requireTestsRan(t, out, 3)
	if pct < 80.0 {
		t.Errorf("cmd/evolve/cmdutil coverage = %.1f%%, want >= 80.0%%\n%s", pct, out)
	}
}

// TestC549_002_CommitgateCoverage_ClearsBar: internal/commitgate (lanePython,
// isPyTest, laneNode, laneRust — all 0.0% before this cycle, the "non-Go lane
// fixtures/removal" clause) is committed to >=80% package coverage. Drives
// lanes_test.go + commitgate_test.go's existing laneGo/reviewer/attestation
// tests.
func TestC549_002_CommitgateCoverage_ClearsBar(t *testing.T) {
	pct, out := runCoverage(t, "github.com/mickeyyaya/evolve-loop/go/internal/commitgate", ".")
	requireTestsRan(t, out, 15)
	if pct < 80.0 {
		t.Errorf("internal/commitgate coverage = %.1f%%, want >= 80.0%%\n%s", pct, out)
	}
}

// TestC549_003_WorktreeSwarmFunctions_FixtureCovered: the NAMED gap
// (runWorktreeCreate/List/Cleanup, runSwarmReap) goes from 0.0% (verified in
// test-report.md's RED Run Output) to real fixture-driven success+error-path
// coverage. Asserts the new tests actually ran and passed (non-degenerate) —
// cmd/evolve is too large a package for a single Medium cycle to lift to an
// 80% PACKAGE-WIDE bar (see test-report.md's Coverage Map "cmd/evolve
// package-wide 80%" row, manual+checklist), so this predicate targets the
// committed NAMED functions via the new test names, not the whole-package
// percentage.
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
