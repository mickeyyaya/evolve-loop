//go:build acs

// Package cycle329 materializes the cycle-329 acceptance criteria for the two
// committed top_n tasks (scout-report.md "## Selected Tasks"):
//
//	verdictcache-write-error-coverage — raise go/internal/verdictcache statement
//	    coverage from the 79.5% baseline to the committed >= 93.0% floor by
//	    exercising the dark error exits of (*Store).write (mkdir + write-temp)
//	    and (*Store).Load (the non-IsNotExist read-error arm), plus the
//	    NewStore nil-now default. The json.MarshalIndent and os.Rename error
//	    arms of write are not in the committed plan, so ~82% — not 100% — is the
//	    practical write() ceiling and the C329_003 floor is set accordingly.
//
//	triagecap-uncovered-fns-coverage — raise go/internal/triagecap statement
//	    coverage from the 86.8% baseline to the committed >= 93.0% floor by
//	    adding direct tests for the four 0.0%-coverage functions: NewReviewer,
//	    readFailedApproaches, CommittedFloorPackages, readWindow.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The coverage gates RUN the
// real package suites in a subprocess under -coverprofile and assert on the
// measured `go tool cover -func` percentages; the suite gates RUN the suites
// under -race and assert on the measured exit code. There is no load-bearing
// source-grep:
//
//   - A magic string in a source file cannot move a coverage number. Only
//     Builder's new tests can — so the coverage gates are anti-no-op by
//     construction.
//   - An EMPTY repo (no package tests) yields 0% and fails every floor.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, so every coverage gate folds in the no-regression axis.
//   - C329_006 is the explicit zero-coverage axis (adversarial-testing SKILL
//     §6): each of the four named functions is at 0.0% today, so the gate is RED
//     until Builder's direct tests exercise them — a package-% game that left
//     any of the four dark could not satisfy it.
//
// AC map (1:1 with the scout-report.md per-task "Acceptance Criteria" — the
// "no production code modified" / "only the new test file is added" criteria are
// dispositioned manual+checklist for the Auditor in test-report.md, not as a
// fragile git-diff predicate whose result depends on the harness's phase-commit
// timing):
//
//	verdictcache-write-error-coverage
//	  AC1 package coverage >= 93.0%            → C329_001
//	  AC2 all tests pass (incl. -race)         → C329_002
//	  (anchor) write() error exits exercised   → C329_003
//	triagecap-uncovered-fns-coverage
//	  AC1 package coverage >= 93.0%            → C329_004
//	  AC2 all tests pass (incl. -race)         → C329_005
//	  (anchor) the four 0%-cov funcs exercised → C329_006
//
// Floor binding (R9.3): internal/verdictcache and internal/triagecap are the two
// committed top_n tasks this cycle, so the coverage floors bind committed
// packages. The scout-DEFERRED items (triagecap.init / newCapReviewer,
// adapters/bridge, adapters/ledger, releasepipeline) get ZERO predicates here —
// a floor on a deferred task would starve the committed ones (cycle-280 lesson).
package cycle329

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

const (
	verdictcachePkg = "./internal/verdictcache/..."
	triagecapPkg    = "./internal/triagecap/..."
)

// tail returns the last n lines of s (keeps RED failure messages readable).
func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// runGoTest runs `go test -C <goDir> <args...>` via the shared acsassert helper
// and returns the combined output and exit code. It does not Fatal on a non-zero
// test exit (callers assert on that); acsassert.SubprocessOutput only errors when
// the toolchain itself fails to launch.
func runGoTest(t *testing.T, args ...string) (combined string, code int) {
	t.Helper()
	full := append([]string{"test", "-C", goDir(t)}, args...)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", full...)
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, tail(stderr, 30))
	}
	return stdout + stderr, code
}

// coverTotalRe matches the trailing `total:  (statements)  NN.N%` line.
var coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)

// coverFuncOutput runs the REAL package suite under -coverprofile and returns
// the `go tool cover -func` report. Fatals (RED) if the suite does not compile
// or a test FAILs — a coverage number is only meaningful over a green suite, so
// this folds in the no-regression axis.
func coverFuncOutput(t *testing.T, pkg string) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "c.cover")
	combined, code := runGoTest(t, "-count=1", "-coverprofile="+profile, pkg)
	if code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d) — add the committed coverage tests / fix regressions:\n%s",
			pkg, code, tail(combined, 40))
	}
	stdout, stderr, code2, err := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+profile)
	if err != nil || code2 != 0 {
		t.Fatalf("go tool cover failed (exit=%d, err=%v):\n%s", code2, err, tail(stderr, 20))
	}
	return stdout
}

// totalCoverage parses the `total:` percentage; Fatals if absent.
func totalCoverage(t *testing.T, out string) float64 {
	t.Helper()
	m := coverTotalRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("could not parse total coverage from:\n%s", tail(out, 20))
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("unparsable total coverage %q: %v", m[1], err)
	}
	return pct
}

// funcCoverage parses the per-function percentage for fn from the
// `<file>:<line>:\t<fn>\t<pct>%` lines; Fatals if the function is absent. The
// `:\d+:` anchor binds fn to a real func-coverage row (not a substring of
// another symbol).
func funcCoverage(t *testing.T, out, fn string) float64 {
	t.Helper()
	re := regexp.MustCompile(`(?m)^\S+:\d+:\s+` + regexp.QuoteMeta(fn) + `\s+([0-9.]+)%`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("could not find %s() in coverage output (renamed/removed?):\n%s", fn, tail(out, 20))
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("unparsable %s coverage %q: %v", fn, m[1], err)
	}
	return pct
}

// =============== verdictcache-write-error-coverage predicates ================

// --- C329_001 (AC1): go/internal/verdictcache total coverage >= 93.0% --------
//
// Behavioral coverage-floor over the real verdictcache suite. RED baseline:
// 79.5%. GREEN requires Builder's nil-now, Load read-error, write mkdir-error,
// and write write-temp-error tests. coverFuncOutput Fatals if any existing test
// regresses, so this folds in the no-regression axis too.
func TestC329_001_VerdictcacheCoverageFloor(t *testing.T) {
	const floor = 93.0
	out := coverFuncOutput(t, verdictcachePkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/verdictcache coverage %.1f%% < %.1f%% floor — exercise (*Store).write's "+
			"mkdir / write-temp error exits, (*Store).Load's non-IsNotExist read error, and NewStore's "+
			"nil-now default. Baseline 79.5%%.", pct, floor)
	}
}

// --- C329_002 (AC2): the verdictcache suite stays green under -race -----------
//
// Regression + data-race axis. Runs the full verdictcache suite under -race and
// requires a clean exit. Pre-existing GREEN today (the existing suite passes);
// Builder must KEEP it green while adding the new error-path tests. Distinct
// command verb (-race, no coverprofile) from the coverage gate for lexical
// diversity.
func TestC329_002_VerdictcacheSuiteGreenRace(t *testing.T) {
	combined, code := runGoTest(t, "-race", "-count=1", verdictcachePkg)
	if code != 0 {
		t.Errorf("RED: internal/verdictcache suite fails under -race (exit=%d) — the new error-path "+
			"tests must not break the round-trip / persist / degrade-on-corrupt contract:\n%s",
			code, tail(combined, 30))
	}
}

// --- C329_003 (anchor for AC1): (*Store).write coverage >= 80% ----------------
//
// Behavioral + negative anchor. write is at 63.6% today: the happy path is
// covered via Put, but the mkdir-error and write-temp-error exits are dark.
// Reaching 80% REQUIRES tests that drive write through those two failure exits
// (the marshal-error and rename-error arms are outside the committed plan, so
// ~82% is the practical ceiling — the floor is 80, not 100). This guarantees
// the package % is moved by exercising the TARGET error branches, not by
// covering unrelated code.
func TestC329_003_StoreWriteErrorExitsExercised(t *testing.T) {
	const floor = 80.0
	out := coverFuncOutput(t, verdictcachePkg)
	if pct := funcCoverage(t, out, "write"); pct < floor {
		t.Errorf("RED: (*Store).write coverage %.1f%% < %.1f%% — add tests that fail os.MkdirAll "+
			"(file-as-dir) and os.WriteFile (read-only .evolve dir). Baseline 63.6%%; marshal/rename "+
			"error arms are out of scope so ~82%% is the practical ceiling.", pct, floor)
	}
}

// ============== triagecap-uncovered-fns-coverage predicates ==================

// --- C329_004 (AC1): go/internal/triagecap total coverage >= 93.0% -----------
//
// Behavioral coverage-floor over the real triagecap suite. RED baseline: 86.8%.
// GREEN requires Builder's direct tests for the four 0%-coverage functions.
func TestC329_004_TriagecapCoverageFloor(t *testing.T) {
	const floor = 93.0
	out := coverFuncOutput(t, triagecapPkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/triagecap coverage %.1f%% < %.1f%% floor — add direct tests for "+
			"NewReviewer, readFailedApproaches, CommittedFloorPackages, readWindow. Baseline 86.8%%.", pct, floor)
	}
}

// --- C329_005 (AC2): the triagecap suite stays green under -race -------------
//
// Regression + data-race axis (scout AC2 explicitly requires -race). Runs the
// full triagecap suite under -race and requires a clean exit. Pre-existing GREEN
// today; Builder must KEEP it green while adding the new uncovered_fns_test.go.
func TestC329_005_TriagecapSuiteGreenRace(t *testing.T) {
	combined, code := runGoTest(t, "-race", "-count=1", triagecapPkg)
	if code != 0 {
		t.Errorf("RED: internal/triagecap suite fails under -race (exit=%d) — the new "+
			"uncovered_fns_test.go must not break the floors / demotion / window contract:\n%s",
			code, tail(combined, 30))
	}
}

// --- C329_006 (anchor for AC1): the four 0%-coverage functions are exercised --
//
// Behavioral zero-coverage axis (adversarial-testing SKILL §6). Each of the four
// scout-named functions is at 0.0% today. The gate requires each to clear a 50%
// floor — comfortably reachable by the planned direct tests, but impossible at
// the 0.0% baseline and impossible to satisfy by raising the package % through
// unrelated code. This is the load-bearing anti-no-op binding for task 2: it
// pins the % movement to the TARGET dark functions, not to incidental coverage.
func TestC329_006_TriagecapZeroCovFunctionsExercised(t *testing.T) {
	const floor = 50.0
	out := coverFuncOutput(t, triagecapPkg)
	for _, fn := range []string{"NewReviewer", "readFailedApproaches", "CommittedFloorPackages", "readWindow"} {
		if pct := funcCoverage(t, out, fn); pct < floor {
			t.Errorf("RED: triagecap.%s coverage %.1f%% < %.1f%% — this function was 0.0%% at baseline "+
				"and must be directly exercised by uncovered_fns_test.go.", fn, pct, floor)
		}
	}
}
