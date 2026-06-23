//go:build acs

// Package cycle347 materializes the cycle-347 acceptance criteria for the three
// committed top_n tasks (triage-report.md ## top_n):
//
//	T1  skillcheck-run-coverage — raise go/internal/skillcheck statement coverage
//	    from the 69.2% baseline to >= 70% by adding four TestRun_* tests that
//	    exercise Run(projectRoot, write, stdout, stderr) across: (a) write=false
//	    no-drift → exit 0 + "check OK" on stdout, (b) write=false drift → exit 2
//	    + "DRIFT:" on stderr, (c) write=true drift → file rewritten in-place
//	    exit 0, (d) invalid projectRoot (catalog missing) → exit 1.
//
//	T2  codequality-edge-coverage — raise go/internal/codequality coverage from
//	    86.4% to >= 90% by adding firstLine(s) no-newline + missing-gofmt-binary
//	    tests (t.Setenv("PATH", "")).
//
//	T3  cycle-audit-cycle-scoped-ci-gap (operator inbox HIGH) — add
//	    TestNewDefault_WiresSkillsDriftCheck to audit_skillsdrift_test.go and
//	    cover the Worktree="" fallback path of skillsDriftCheckDefault, so audit
//	    correctly fails cycles with SKILL.md drift from any call site.
//
// Predicates are BEHAVIORAL (cycle-85 lesson). Coverage gates run the real
// suites under -coverprofile and assert on `go tool cover -func` output; the
// behavioral gate (C347_005) runs a specific test function via subprocess and
// asserts on the PASS line presence. No load-bearing source-grep.
//
// AC map (1:1 with triage top_n items):
//
//	T1.coverage     skillcheck package coverage >= 70%           → C347_001
//	T1.run-nonzero  Run function not at 0.0% coverage           → C347_002
//	T2.coverage     codequality package coverage >= 90%         → C347_003
//	T2.firstline    firstLine function at 100% coverage         → C347_004
//	T3.skills-wire  TestNewDefault_WiresSkillsDriftCheck passes → C347_005
//	T3.audit-cover  audit package coverage >= 91%               → C347_006
//
// Floor binding (R9.3): T1/T2/T3 are the three committed top_n tasks this cycle;
// their coverage floors bind committed packages. soakreport (triage-deferred P3)
// gets ZERO predicates here — a floor on a deferred task starves the committed
// ones (cycle-280 lesson).
package cycle347

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
// It Fatals if the suite fails to compile or any test FAILs — so every coverage
// gate folds in the no-regression axis (no separate suite-passes predicate needed).
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
// from `go tool cover -func` output. Returns -1.0 if the function is not found.
func funcCoverage(funcOut, funcName string) float64 {
	// Matches lines like: "...skillcheck.go:135:	Run	0.0%"
	re := regexp.MustCompile(`(?m)\b` + regexp.QuoteMeta(funcName) + `\s+([0-9.]+)%`)
	m := re.FindStringSubmatch(funcOut)
	if m == nil {
		return -1.0
	}
	pct, _ := strconv.ParseFloat(m[1], 64)
	return pct
}

const (
	skillcheckPkg  = "./internal/skillcheck/..."
	codequalityPkg = "./internal/codequality/..."
	auditPkg       = "./internal/phases/audit/..."
)

// TestC347_001_SkillcheckRunCoverageFloor verifies that go/internal/skillcheck
// statement coverage is >= 70% after Builder adds TestRun_* tests.
// BEHAVIORAL: runs the real test suite under -coverprofile and asserts on the
// measured total from `go tool cover -func`. A source-file magic string cannot
// move this number — only Builder's new tests calling Run() can.
// RED: coverage is 69.2% (Run function at 0.0%) before Builder's additions.
func TestC347_001_SkillcheckRunCoverageFloor(t *testing.T) {
	_, pct := coverFuncOutput(t, skillcheckPkg)
	if pct < 70.0 {
		t.Errorf("RED: internal/skillcheck coverage = %.1f%%, want >= 70.0%%\n"+
			"Builder must add TestRun_* tests exercising Run(projectRoot, write, stdout, stderr)\n"+
			"across: write=false no-drift, write=false drift (exit 2), write=true drift, invalid root (exit 1)",
			pct)
	}
}

// TestC347_002_SkillcheckRunFunctionNonZero verifies that the Run exported
// function specifically has non-zero statement coverage (not stuck at 0.0%).
// BEHAVIORAL: parses `go tool cover -func` for the Run function line and
// asserts its percentage is > 0.
// RED: Run is at 0.0% today — none of the existing tests call Run().
// This is the anti-no-op anchor: package% can't cross 70% without Run > 0%
// because Run contains ~36 statements in the package's ~205 total.
func TestC347_002_SkillcheckRunFunctionNonZero(t *testing.T) {
	funcOut, _ := coverFuncOutput(t, skillcheckPkg)
	pct := funcCoverage(funcOut, "Run")
	switch {
	case pct < 0:
		t.Errorf("RED: Run function not found in `go tool cover -func` output\n"+
			"Expected function at go/internal/skillcheck/skillcheck.go:135\n"+
			"Coverage output:\n%s", tail(funcOut, 30))
	case pct == 0.0:
		t.Errorf("RED: Run function coverage = 0.0%% — no test calls Run()\n"+
			"Builder must add tests that call skillcheck.Run(projectRoot, write, stdout, stderr)\n"+
			"Coverage output:\n%s", tail(funcOut, 30))
	}
}

// TestC347_003_CodequalityCoverageFloor verifies that go/internal/codequality
// statement coverage is >= 90% after Builder adds the two missing tests.
// BEHAVIORAL: runs the real codequality suite under -coverprofile.
// RED: coverage is 86.4% (firstLine at 66.7%, missing-gofmt-binary at 0%)
// before Builder adds TestFirstLine_NoNewline and TestUnformattedGoFiles_GofmtMissing.
func TestC347_003_CodequalityCoverageFloor(t *testing.T) {
	_, pct := coverFuncOutput(t, codequalityPkg)
	if pct < 90.0 {
		t.Errorf("RED: internal/codequality coverage = %.1f%%, want >= 90.0%%\n"+
			"Builder must add:\n"+
			"  TestFirstLine_NoNewline — firstLine(s) when s contains no newline\n"+
			"  TestUnformattedGoFiles_GofmtMissing — t.Setenv(\"PATH\", \"\") to hide gofmt binary",
			pct)
	}
}

// TestC347_004_FirstLineFunctionFullyCovered verifies that the unexported
// firstLine helper in codequality is at 100% coverage (both the newline-present
// and newline-absent branches).
// BEHAVIORAL: parses `go tool cover -func` for firstLine and asserts 100%.
// RED: firstLine is at 66.7% today (the no-newline branch is not exercised by
// any existing test — all gofmt stderr fixtures produce multi-line output).
func TestC347_004_FirstLineFunctionFullyCovered(t *testing.T) {
	funcOut, _ := coverFuncOutput(t, codequalityPkg)
	pct := funcCoverage(funcOut, "firstLine")
	switch {
	case pct < 0:
		t.Errorf("RED: firstLine function not found in `go tool cover -func` output\n"+
			"Expected at go/internal/codequality/gofmt.go:61\n"+
			"Coverage output:\n%s", tail(funcOut, 20))
	case pct < 100.0:
		t.Errorf("RED: firstLine coverage = %.1f%%, want 100%%\n"+
			"Builder must add TestFirstLine_NoNewline exercising the return-s branch\n"+
			"(line 64: when strings.IndexByte finds no newline, return s unchanged)\n"+
			"Coverage output:\n%s", pct, tail(funcOut, 20))
	}
}

// TestC347_005_NewDefaultWiresSkillsDriftCheckTestPasses verifies that
// TestNewDefault_WiresSkillsDriftCheck exists in audit_skillsdrift_test.go and
// passes. This is the behavioral end-to-end gate that confirms NewDefault wires
// the real skillsDriftCheck — the analog of the existing TestNewDefault_WiresGofmtCheck
// for the skills-drift gate (the "skill-gen trigger" criterion from the operator inbox).
// BEHAVIORAL: runs `go test -v -run TestNewDefault_WiresSkillsDriftCheck` and asserts
// the PASS line appears (a missing test exits 0 but emits no PASS line → predicate FAILS).
// RED: TestNewDefault_WiresSkillsDriftCheck does not exist yet →
// `go test -v -run ...` returns PASS with "no tests to run", not a PASS line for
// the function, so the predicate fails.
func TestC347_005_NewDefaultWiresSkillsDriftCheckTestPasses(t *testing.T) {
	dir := goDir(t)
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1", "-v",
		"-run", "TestNewDefault_WiresSkillsDriftCheck",
		"./internal/phases/audit/...",
	)
	if err != nil && code != 0 {
		t.Fatalf("go test subprocess error (exit=%d): %v\nOutput:\n%s", code, err, tail(out, 30))
	}
	passRe := regexp.MustCompile(`(?m)^--- PASS: TestNewDefault_WiresSkillsDriftCheck`)
	if !passRe.MatchString(out) {
		t.Errorf("RED: TestNewDefault_WiresSkillsDriftCheck not found as PASS (exit=%d)\n"+
			"Builder must add TestNewDefault_WiresSkillsDriftCheck to\n"+
			"go/internal/phases/audit/audit_skillsdrift_test.go\n"+
			"It should mirror TestNewDefault_WiresGofmtCheck: create a worktree with a\n"+
			"drifted SKILL.md (mutated generated region), pre-stage acs-verdict.json red=0,\n"+
			"run NewDefault audit, assert Verdict=FAIL with a skills-drift diagnostic.\n"+
			"Use testing.Short() guard (same as the gofmt analog).\n"+
			"Output:\n%s",
			code, tail(out, 30))
	}
}

// TestC347_006_AuditPackageCoverageFloor verifies that go/internal/phases/audit
// statement coverage is >= 91% after Builder adds TestNewDefault_WiresSkillsDriftCheck
// and tests for the Worktree="" fallback paths in gofmtCheckDefault /
// skillsDriftCheckDefault (currently at 66.7% each).
// BEHAVIORAL: runs the real audit suite under -coverprofile.
// RED: audit package coverage is 88.6% before Builder's additions.
// Negative contract: the fallback paths (Worktree="" → use ProjectRoot) exist but
// are untested today; any implementation that leaves them at 0% cannot reach 91%.
func TestC347_006_AuditPackageCoverageFloor(t *testing.T) {
	_, pct := coverFuncOutput(t, auditPkg)
	if pct < 91.0 {
		t.Errorf("RED: internal/phases/audit coverage = %.1f%%, want >= 91.0%%\n"+
			"Builder must add:\n"+
			"  TestNewDefault_WiresSkillsDriftCheck — end-to-end skills-drift gate verification\n"+
			"  Tests for gofmtCheckDefault/skillsDriftCheckDefault Worktree=\"\" fallback paths\n"+
			"  (these are at 66.7%% each today; the fallback to ProjectRoot is uncovered)",
			pct)
	}
}
