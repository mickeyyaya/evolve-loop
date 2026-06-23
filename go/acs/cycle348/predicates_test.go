//go:build acs

// Package cycle348 materializes the cycle-348 acceptance criteria for the two
// committed top_n tasks (triage-report.md ## top_n):
//
//	T1  skillcheck-coverage — raise go/internal/skillcheck statement coverage
//	    from 82.7% to >= 90.0% by adding table-driven unit tests for:
//	    nameMismatches (missing skills dir, invalid frontmatter, name!=dir),
//	    parallelSubtaskCount (nil raw, invalid JSON, array value), and
//	    inspect edge cases.
//
//	T2  rollback-default-fns-coverage — raise go/internal/rollback statement
//	    coverage from 86.8% to >= 92.0% by adding tests for deleteRemoteTagWith
//	    and revertAndShipWith using the existing gitexec.Fake seam, plus smoke
//	    tests that call the one-liner default* wrappers.
//
// Predicates are BEHAVIORAL (cycle-85 lesson). Coverage gates run the real
// suites under -coverprofile and assert on `go tool cover -func` output.
// No load-bearing source-grep.
//
// AC map (1:1 with triage top_n items):
//
//	T1.coverage        skillcheck package coverage >= 90.0%          → C348_001
//	T1.namemismatches  nameMismatches function coverage >= 90.0%     → C348_002
//	T2.coverage        rollback package coverage >= 92.0%            → C348_003
//	T2.revertship      revertAndShipWith function coverage >= 80.0%  → C348_004
//
// Floor binding (R9.3): T1/T2 are the two committed top_n tasks this cycle.
// Deferred items (ship-phase-no-deliverable-contract, bridge-integration-tests)
// get ZERO predicates — a floor on a deferred task starves the committed ones
// (cycle-280 lesson).
package cycle348

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// goDir returns the go module directory; `-C goDir` makes every subprocess
// invocation cwd-independent (the audit lane may run from the worktree root).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// tail returns the last n lines of s for readable RED failure messages.
func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// coverFuncOutput runs the real test suite for pkg under -coverprofile and
// returns the `go tool cover -func` output plus the total coverage percentage.
// Fatals if the suite fails to compile or any test FAILs, folding the
// no-regression axis into every coverage gate.
func coverFuncOutput(t *testing.T, pkg string) (funcOut string, totalPct float64) {
	t.Helper()
	dir := goDir(t)
	tmp := t.TempDir()
	cp := filepath.Join(tmp, "c.out")
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1",
		"-coverprofile="+cp,
		"-coverpkg="+pkg,
		pkg,
	)
	if code != 0 || err != nil {
		t.Fatalf("go test %s failed (exit=%d): %v\nOutput:\n%s", pkg, code, err, tail(out, 40))
	}
	funcOut2, _, _, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+cp)
	totalRe := regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)
	m := totalRe.FindStringSubmatch(funcOut2)
	if m == nil {
		t.Fatalf("could not parse total from `go tool cover -func`:\n%s", funcOut2)
	}
	pct, _ := strconv.ParseFloat(m[1], 64)
	return funcOut2, pct
}

// funcCoverage extracts the coverage percentage for a specific function name
// from `go tool cover -func` output. Returns -1.0 when not found.
func funcCoverage(funcOut, funcName string) float64 {
	re := regexp.MustCompile(`(?m)\b` + regexp.QuoteMeta(funcName) + `\s+([0-9.]+)%`)
	m := re.FindStringSubmatch(funcOut)
	if m == nil {
		return -1.0
	}
	pct, _ := strconv.ParseFloat(m[1], 64)
	return pct
}

const (
	skillcheckPkg = "./internal/skillcheck/..."
	rollbackPkg   = "./internal/rollback/..."
)

// TestC348_001_SkillcheckCoverageFloor verifies that go/internal/skillcheck
// statement coverage is >= 90.0% after Builder adds table-driven tests for
// nameMismatches (missing-dir, invalid-frontmatter, name!=dir branches) and
// parallelSubtaskCount (nil, invalid JSON, array branches).
// BEHAVIORAL: runs the real skillcheck test suite under -coverprofile and
// asserts on the measured total from `go tool cover -func`.
// RED: coverage is 82.7% before Builder's additions (nameMismatches at 68.4%,
// parallelSubtaskCount at 66.7%, inspect at 78.6%).
func TestC348_001_SkillcheckCoverageFloor(t *testing.T) {
	_, pct := coverFuncOutput(t, skillcheckPkg)
	if pct < 90.0 {
		t.Errorf("RED: internal/skillcheck coverage = %.1f%%, want >= 90.0%%\n"+
			"Builder must add tests exercising:\n"+
			"  nameMismatches: (a) missing skills dir → 'read skills dir' error,\n"+
			"    (b) invalid frontmatter YAML → 'unparseable' message,\n"+
			"    (c) name != dir → DRIFT message\n"+
			"  parallelSubtaskCount: (a) nil/zero raw → 0, (b) invalid JSON → 0,\n"+
			"    (c) array value → len(array)\n"+
			"  inspect: non-skill dir (no SKILL.md) edge case",
			pct)
	}
}

// TestC348_002_SkillcheckNameMismatchesCovered verifies that the unexported
// nameMismatches function has statement coverage >= 90.0% — meaning Builder
// exercised the three previously-uncovered branches:
//   - os.ReadDir failure (missing skills dir)
//   - prompts.ParseFrontmatter error (invalid YAML frontmatter)
//   - name != dir (drift) case
//
// BEHAVIORAL: parses `go tool cover -func` for the nameMismatches line.
// A source-file magic string cannot move this number — only new tests calling
// nameMismatches with controlled fixtures can.
// RED: nameMismatches is at 68.4% today (three branches uncovered).
func TestC348_002_SkillcheckNameMismatchesCovered(t *testing.T) {
	funcOut, _ := coverFuncOutput(t, skillcheckPkg)
	pct := funcCoverage(funcOut, "nameMismatches")
	switch {
	case pct < 0:
		t.Errorf("RED: nameMismatches not found in `go tool cover -func` output\n"+
			"Expected in go/internal/skillcheck/skillcheck.go:207\n"+
			"Coverage output:\n%s", tail(funcOut, 30))
	case pct < 90.0:
		t.Errorf("RED: nameMismatches coverage = %.1f%%, want >= 90.0%%\n"+
			"Builder must add tests for: missing-skills-dir, invalid-frontmatter,\n"+
			"and name!=dir branches. All three branches require a temp-dir fixture.\n"+
			"Coverage output:\n%s", pct, tail(funcOut, 30))
	}
}

// TestC348_003_RollbackCoverageFloor verifies that go/internal/rollback
// statement coverage is >= 92.0% after Builder adds tests for
// deleteRemoteTagWith and revertAndShipWith via the existing gitexec seam.
// BEHAVIORAL: runs the real rollback test suite under -coverprofile.
// RED: coverage is 86.8% before Builder's additions (revertAndShipWith at
// 54.5%, defaultDeleteRemoteTag and defaultRevertAndShip at 0.0%,
// appendLedger at 76.9%).
func TestC348_003_RollbackCoverageFloor(t *testing.T) {
	_, pct := coverFuncOutput(t, rollbackPkg)
	if pct < 92.0 {
		t.Errorf("RED: internal/rollback coverage = %.1f%%, want >= 92.0%%\n"+
			"deleteRemoteTagWith is already at 100%% — the gaps are elsewhere:\n"+
			"  revertAndShipWith: add tests where the fake binary exits 1 (→ 'local-only')\n"+
			"    and exits 0 (→ 'reverted') via EVOLVE_GO_BIN pointing to a temp script\n"+
			"  defaultDeleteRemoteTag (0%%): smoke-call with a temp non-git dir (→ 'not-present')\n"+
			"  defaultRevertAndShip (0%%): smoke-call with a temp non-git dir (→ 'failed')\n"+
			"  appendLedger (76.9%%): exercise the write-error path (parent is a file)",
			pct)
	}
}

// TestC348_004_RollbackRevertAndShipWithCovered verifies that the unexported
// revertAndShipWith function has statement coverage >= 80.0% — meaning Builder
// exercised the two uncovered branches:
//   - binary exists but ship command exits non-zero → "local-only"
//   - binary exists and ship command exits 0 → "reverted"
//
// BEHAVIORAL: parses `go tool cover -func` for the revertAndShipWith line.
// RED: revertAndShipWith is at 54.5% today; the existing test only exercises
// the revert-fails and no-binary-found paths, leaving the ship-cmd branches
// unreachable.
// Anti-no-op: the "reverted" return is in a code path that requires an actual
// executable binary — a source-text addition cannot fake exec.Command succeeding.
func TestC348_004_RollbackRevertAndShipWithCovered(t *testing.T) {
	funcOut, _ := coverFuncOutput(t, rollbackPkg)
	pct := funcCoverage(funcOut, "revertAndShipWith")
	switch {
	case pct < 0:
		t.Errorf("RED: revertAndShipWith not found in `go tool cover -func` output\n"+
			"Expected in go/internal/rollback/rollback.go:361\n"+
			"Coverage output:\n%s", tail(funcOut, 30))
	case pct < 80.0:
		t.Errorf("RED: revertAndShipWith coverage = %.1f%%, want >= 80.0%%\n"+
			"Builder must add tests exercising exec.Command(binPath, \"ship\", ...):\n"+
			"  (a) t.Setenv(\"EVOLVE_GO_BIN\", fakeBin) where fakeBin exits 1 → 'local-only'\n"+
			"  (b) t.Setenv(\"EVOLVE_GO_BIN\", fakeBin) where fakeBin exits 0 → 'reverted'\n"+
			"fakeBin = a temp script with #!/bin/sh + exit 0/1 written to t.TempDir()\n"+
			"Coverage output:\n%s", pct, tail(funcOut, 30))
	}
}
