//go:build acs

// Package cycle299 materializes the cycle-299 acceptance criteria for the three
// committed top_n tasks (scout-report.md / triage-report.md — coverage +
// reliability hardening of the concurrency campaign's leaf packages):
//
//	T1  flock-tests — add the first test file for internal/adapters/flock (the
//	    blocking cross-process lock that serializes ledger.Append CA.1 +
//	    storage.UpdateState CA.3). Two sub-criteria:
//	      (a) package statement coverage ≥ 90.0% (0% baseline — no test file).
//	      (b) the flockFn error branch is tested — encoded as Lock-function
//	          completeness: 100% of Lock means EVERY error return (MkdirAll,
//	          OpenFile, and the flockFn LOCK_EX seam) plus the release closure
//	          are exercised. Lock is the package's only function, so the floor
//	          and the completeness gate read the same number through different
//	          thresholds: 90% admits one missed error return, 100% admits none —
//	          and only the flockFn-error return + the release path push it from
//	          ~92% to 100%, so this gate is what pins the seam branch.
//	T2  runlease-write-error-coverage — extend runlease_test.go to cover Write's
//	    four atomic-write error paths (CreateTemp/Write/Close/Rename). Two
//	    sub-criteria: (a) package total ≥ 95.0% (70.6% baseline), (b) the Write
//	    function ≥ 80.0% (52.6% baseline — happy-path round-trip only today).
//	T3  sessionrecord-coverage-boost — extend sessionrecord_test.go. Two
//	    sub-criteria: (a) package total ≥ 90.0% (68.8% baseline), (b) the
//	    RunScopeToken function ≥ 90.0% (0.0% baseline — never tested; 90% forces
//	    the >8-char ULID truncation branch, scout hypothesis 3's boundary).
//
// These predicates are BEHAVIORAL (cycle-85 lesson) and follow the canonical
// coverage-floor shape proven in cycle298's C298_003: each one RUNS the real
// package test suite under -coverprofile (a real subprocess exercising real
// code paths) and gates on the measured percentage from `go tool cover -func`.
// A magic string in a source file cannot satisfy them — only Builder's new
// tests, which drive the real error branches, move the number. An EMPTY repo
// (no tests) yields 0% and fails every gate, so they are anti-no-op by
// construction. Each floor also Fatals if the package suite is not green
// (exit != 0), folding in a no-regression axis.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary", 6 rows):
//
//	T1(a) flock total coverage ≥ 90%              → C299_001
//	T1(b) flockFn error branch tested (Lock=100%) → C299_002
//	T2(a) runlease total coverage ≥ 95%           → C299_003
//	T2(b) runlease Write coverage ≥ 80%           → C299_004
//	T3(a) sessionrecord total coverage ≥ 90%      → C299_005
//	T3(b) sessionrecord RunScopeToken ≥ 90%       → C299_006
//
// Floor binding (R9.3): all three packages are committed top_n tasks THIS cycle
// (triage-report.md ## top_n) — no deferred-floor predicate here.
package cycle299

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
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
		t.Fatalf("RED: %s test run failed (exit=%d, err=%v) — add the new test file / fix regressions:\n%s",
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
// another symbol) — e.g. searching "Read" never matches the "ReadAll" row.
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
	flockPkg    = "./internal/adapters/flock/..."
	runleasePkg = "./internal/runlease/..."
	sessionPkg  = "./internal/sessionrecord/..."
)

// ============================ T1 — flock-tests ===============================

// --- C299_001 (T1a): internal/adapters/flock total coverage ≥ 90.0% ----------
//
// Behavioral coverage-floor: runs the real flock suite under -coverprofile and
// gates on the measured total. RED baseline: 0.0% (no flock_test.go exists).
// GREEN requires Builder's new flock_test.go exercising Lock + its error
// branches. No source string fakes a measured percentage.
func TestC299_001_FlockCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t, flockPkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/adapters/flock coverage %.1f%% < %.1f%% floor — "+
			"add flock_test.go covering Lock + MkdirAll/OpenFile/flockFn error branches", pct, floor)
	}
}

// --- C299_002 (T1b): the flockFn error branch is tested (Lock = 100%) ---------
//
// Behavioral: Lock is flock.go's only function, holding three error returns
// (MkdirAll, OpenFile, the flockFn LOCK_EX seam) and the LOCK_UN release
// closure. A happy-path+release test alone reaches only ~77%; covering the
// MkdirAll + OpenFile error returns lifts it to ~92%; ONLY exercising the
// flockFn-error return (via the `var flockFn` seam) and the release path push
// Lock to 100%. So a 100% Lock floor is the precise, behavioral encoding of
// "flockFn error branch tested" — strictly stronger than the 90% aggregate
// gate, and unsatisfiable without injecting a flockFn error. RED baseline:
// 0.0%.
func TestC299_002_FlockErrorBranchesCovered(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t, flockPkg)
	if pct := funcCoverage(t, out, "Lock"); pct < floor {
		t.Errorf("RED: flock.Lock coverage %.1f%% < %.1f%% — the flockFn LOCK_EX error "+
			"branch (and/or the release closure) is not exercised; inject an error via the "+
			"`var flockFn` seam and assert Lock returns it", pct, floor)
	}
}

// ===================== T2 — runlease-write-error-coverage ====================

// --- C299_003 (T2a): internal/runlease total coverage ≥ 95.0% ----------------
//
// Behavioral coverage-floor over the real runlease suite. RED baseline: 70.6%.
// GREEN requires Builder's new Write error-path tests (CreateTemp/Write/Close/
// Rename failures).
func TestC299_003_RunleaseCoverageFloor(t *testing.T) {
	const floor = 95.0
	out := coverFuncOutput(t, runleasePkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/runlease coverage %.1f%% < %.1f%% floor — "+
			"cover Write's CreateTemp/Write/Close/Rename error returns", pct, floor)
	}
}

// --- C299_004 (T2b): runlease.Write coverage ≥ 80.0% -------------------------
//
// Behavioral per-function floor. Write drives the GC liveness heartbeat; its
// four atomic-write error returns are all dark today (the existing test is a
// happy-path round-trip). RED baseline: 52.6%. 80% forces the majority of the
// error branches to be exercised (the happy path alone caps near 53%).
func TestC299_004_RunleaseWriteCoverage(t *testing.T) {
	const floor = 80.0
	out := coverFuncOutput(t, runleasePkg)
	if pct := funcCoverage(t, out, "Write"); pct < floor {
		t.Errorf("RED: runlease.Write coverage %.1f%% < %.1f%% — exercise the tmp/write/"+
			"close/rename error returns (e.g. a non-writable runDir, a rename collision)", pct, floor)
	}
}

// ===================== T3 — sessionrecord-coverage-boost =====================

// --- C299_005 (T3a): internal/sessionrecord total coverage ≥ 90.0% -----------
//
// Behavioral coverage-floor over the real sessionrecord suite. RED baseline:
// 68.8%. GREEN requires Builder's RunScopeToken + Append error-branch tests.
func TestC299_005_SessionrecordCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t, sessionPkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/sessionrecord coverage %.1f%% < %.1f%% floor — "+
			"test RunScopeToken edge cases + Append open/write/close error branches", pct, floor)
	}
}

// --- C299_006 (T3b): sessionrecord.RunScopeToken coverage ≥ 90.0% ------------
//
// Behavioral per-function floor. RunScopeToken scopes tmux session names and is
// at 0.0% — never tested. Its only branch is the `len(runID) > 8` ULID
// truncation; 90% forces a >8-char input (scout hypothesis 3's boundary) to be
// exercised alongside the short-input path. RED baseline: 0.0%.
func TestC299_006_SessionrecordRunScopeTokenCoverage(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t, sessionPkg)
	if pct := funcCoverage(t, out, "RunScopeToken"); pct < floor {
		t.Errorf("RED: sessionrecord.RunScopeToken coverage %.1f%% < %.1f%% — test the "+
			">8-char truncation branch and the short-input path", pct, floor)
	}
}
