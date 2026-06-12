//go:build acs

// Package cycle300 materializes the cycle-300 acceptance criteria for the two
// committed top_n tasks (scout-report.md / triage-report.md — error-path +
// safety-gate coverage of the post-L3.4 reliability surface):
//
//	T1  gc-error-path-coverage — add targeted tests for internal/gc's dark error
//	    paths. Two sub-criteria (scout "Acceptance Criteria Summary"):
//	      (a) the 5 new error-path tests exercise their branches — encoded as
//	          Plan = 100% AND dirEntriesOlderThan = 100%. Plan's only dark blocks
//	          are the EvolveDir-absolute guard (gc.go:125-127) and the nil-Now
//	          wall-clock fallback (gc.go:130-132); dirEntriesOlderThan's only dark
//	          block is the filter-reject callback (gc.go:204-205). Only the guard /
//	          nil-Now / filter-reject tests push these two functions to 100%, so a
//	          dual 100% floor is the precise behavioral encoding of "the error-path
//	          tests exist and drive their branches" — strictly stronger than
//	          "5 tests pass", unsatisfiable by a no-op.
//	      (b) package statement coverage >= 98.0% (95.0% baseline). The 98% floor
//	          additionally forces the Discover missing-runs branch (discover.go:
//	          81-83) and the currentWorkspace read-error branch (discover.go:
//	          141-143) — 5 of the 8 coverable dark blocks (the remaining 3 —
//	          e.Info race, Apply RemoveAll/Rename — are triage-deferred, so 98.14%
//	          is the reachable ceiling and 98.0% leaves a one-block margin).
//	T2  looppreflight-coverage-boost — add targeted tests for the loop readiness
//	    gate's dark safety seams. Two sub-criteria:
//	      (a) the 6 new safety-gate tests exercise their branches — encoded as
//	          CheckLevel.String = 100% (the out-of-range "unknown" branch,
//	          looppreflight.go:60-61) AND checkPipelineStructure = 100% (the three
//	          Halt-accumulation gaps: missing factory gc.go... checks.go:30-32,
//	          profileLister error checks.go:39-41, profileGetter error checks.go:
//	          44-46). checkPipelineStructure is THE gate that refuses to start a
//	          loop whose static wiring is broken, so pinning it to 100% guarantees
//	          every gap-accumulation arm is driven.
//	      (b) package statement coverage >= 93.0% (88.7% baseline). The deferred
//	          newDefaultBootTester closure (14 dark stmts, needs an fs seam) and
//	          the dead PrettyJSON error branch are excluded, leaving 25 coverable
//	          dark stmts (95.76% ceiling); 93.0% needs 16 of them.
//
// These predicates are BEHAVIORAL (cycle-85 lesson) and follow the canonical
// coverage-floor shape proven in cycle298/299: each one RUNS the real package
// test suite under -coverprofile (a real subprocess exercising real code paths)
// and gates on the measured percentage from `go tool cover -func`. A magic
// string in a source file cannot satisfy them — only Builder's new tests, which
// drive the real error branches, move the number. An EMPTY repo (no tests)
// yields 0% and fails every gate, so they are anti-no-op by construction. Each
// floor also Fatals if the package suite is not green (exit != 0), folding in a
// no-regression axis.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary", 4 rows):
//
//	T1(a) 5 new gc tests pass  (Plan=100% ∧ dirEntriesOlderThan=100%) → C300_001
//	T1(b) gc total coverage >= 98%                                    → C300_002
//	T2(a) 6 new lpf tests pass (String=100% ∧ checkPipelineStructure=100%) → C300_003
//	T2(b) looppreflight total coverage >= 93%                         → C300_004
//
// Floor binding (R9.3): both packages are committed top_n tasks THIS cycle
// (triage-report.md ## top_n). The triage-deferred items (gc-toctou-delete-error,
// looppreflight-boot-tester-integration) get ZERO predicates here — the floors
// are reachable WITHOUT their branches (gc 98.14% / lpf 95.76% ceilings).
package cycle300

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

// coverTotalRe matches the trailing `total:  (statements)  NN.N%` line of
// `go tool cover -func`.
var coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// coverFuncOutput runs the REAL package test suite under -coverprofile and
// returns the `go tool cover -func` report. Fatals (RED) if the suite does not
// compile or a test FAILs — the floor predicates require a green suite before a
// coverage number is meaningful.
func coverFuncOutput(t *testing.T, pkg string) string {
	t.Helper()
	dir := goDir(t)
	profile := filepath.Join(t.TempDir(), "c.cover")
	if _, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "-coverprofile="+profile, pkg); err != nil || code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d, err=%v) — add the new tests / fix regressions:\n%s",
			pkg, code, err, tail(stderr, 40))
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover failed (exit=%d, err=%v):\n%s", code, err, tail(stderr, 20))
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
// `<file>:<line>:\t<fn>\t<pct>%` lines; Fatals if the function is absent.
// The `:\d+:` anchor binds fn to a real func-coverage row (not a substring of
// another symbol) — e.g. searching "Plan" never matches a "Planner" row.
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

const (
	gcPkg  = "./internal/gc/..."
	lpfPkg = "./internal/looppreflight/..."
)

// ======================= T1 — gc-error-path-coverage =========================

// --- C300_001 (T1a): the 5 gc error-path tests drive their branches ----------
//
// Behavioral per-function floor. internal/gc's error paths are dark today:
// Plan's EvolveDir-absolute guard (gc.go:125-127) and nil-Now wall-clock
// fallback (gc.go:130-132) leave Plan at 94.4%; dirEntriesOlderThan's
// filter-reject callback (gc.go:204-205) leaves it at 92.9%. Driving exactly
// those branches — a relative/empty EvolveDir (a NEGATIVE test that must be
// rejected), a nil opts.Now, and a filter that rejects an entry — lifts both
// functions to 100%. A 100% floor on BOTH is the precise behavioral encoding of
// "the 5 error-path tests exist and exercise their branches": no source string
// fakes it, and a no-op build leaves them below 100%. RED baseline: Plan 94.4%,
// dirEntriesOlderThan 92.9%.
func TestC300_001_GCErrorPathBranchesCovered(t *testing.T) {
	out := coverFuncOutput(t, gcPkg)
	if pct := funcCoverage(t, out, "Plan"); pct < 100.0 {
		t.Errorf("RED: gc.Plan coverage %.1f%% < 100%% — cover the EvolveDir-absolute "+
			"guard (relative/empty path rejected) and the nil-Now wall-clock fallback", pct)
	}
	if pct := funcCoverage(t, out, "dirEntriesOlderThan"); pct < 100.0 {
		t.Errorf("RED: gc.dirEntriesOlderThan coverage %.1f%% < 100%% — exercise the "+
			"filter-reject callback (an entry the filter returns false for must be skipped)", pct)
	}
}

// --- C300_002 (T1b): internal/gc total coverage >= 98.0% ---------------------
//
// Behavioral coverage-floor over the real gc suite. RED baseline: 95.0%. GREEN
// requires Builder's new tests for the Plan guards, nil-Now, the filter-reject
// callback, Discover's missing-runs branch (discover.go:81-83) AND the
// currentWorkspace read-error branch (discover.go:141-143) — 5 of the 8 coverable
// dark blocks. The other 3 (e.Info race, Apply RemoveAll/Rename) are
// triage-deferred, so 98.14% is the ceiling; this floor leaves one block of
// margin.
func TestC300_002_GCCoverageFloor(t *testing.T) {
	const floor = 98.0
	out := coverFuncOutput(t, gcPkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/gc coverage %.1f%% < %.1f%% floor — cover the Plan guards, "+
			"nil-Now, the dirEntriesOlderThan filter-reject, Discover missing-runs, and the "+
			"currentWorkspace read-error branch", pct, floor)
	}
}

// ==================== T2 — looppreflight-coverage-boost =======================

// --- C300_003 (T2a): the 6 looppreflight safety-gate tests drive their branches
//
// Behavioral per-function floor on the loop readiness gate's two darkest safety
// seams. CheckLevel.String's out-of-range "unknown" arm (looppreflight.go:60-61)
// leaves it at 80%; covering it (cast an out-of-range int to CheckLevel) →
// 100%. checkPipelineStructure — the Halt check that refuses to start a loop
// with broken static wiring — leaves three gap-accumulation arms dark
// (missing-factory checks.go:30-32, profileLister error 39-41, profileGetter
// error 44-46), holding it at 81%; injecting a false factoryLookup, an erroring
// profileLister, and an erroring profileGetter drives all three → 100%. A 100%
// floor on BOTH is the behavioral encoding of "the 6 safety-gate tests exist and
// exercise their branches". RED baseline: String 80.0%, checkPipelineStructure
// 81.0%.
func TestC300_003_LoopPreflightSafetyGateBranchesCovered(t *testing.T) {
	out := coverFuncOutput(t, lpfPkg)
	if pct := funcCoverage(t, out, "String"); pct < 100.0 {
		t.Errorf("RED: looppreflight.CheckLevel.String coverage %.1f%% < 100%% — cover the "+
			"out-of-range \"unknown\" branch (cast an int outside LevelPass/Warn/Halt)", pct)
	}
	if pct := funcCoverage(t, out, "checkPipelineStructure"); pct < 100.0 {
		t.Errorf("RED: looppreflight.checkPipelineStructure coverage %.1f%% < 100%% — drive "+
			"all three Halt-accumulation arms (false factoryLookup, erroring profileLister, "+
			"erroring profileGetter)", pct)
	}
}

// --- C300_004 (T2b): internal/looppreflight total coverage >= 93.0% ----------
//
// Behavioral coverage-floor over the real looppreflight suite. RED baseline:
// 88.7%. GREEN requires Builder's new safety-gate tests (String unknown, the
// defaultDiskFreeBytes Statfs-error path via a bogus path, checkPipelineStructure
// gaps, resolve default seams, sandboxWanted no-sandbox-profiles). The deferred
// newDefaultBootTester closure (14 dark stmts) and the dead PrettyJSON error
// branch are excluded, leaving a 95.76% ceiling; 93% leaves margin.
func TestC300_004_LoopPreflightCoverageFloor(t *testing.T) {
	const floor = 93.0
	out := coverFuncOutput(t, lpfPkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/looppreflight coverage %.1f%% < %.1f%% floor — cover String "+
			"unknown, defaultDiskFreeBytes Statfs-error, checkPipelineStructure gaps, resolve "+
			"default seams, and sandboxWanted no-sandbox-profiles", pct, floor)
	}
}
