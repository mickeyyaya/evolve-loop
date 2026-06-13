//go:build acs

// Package cycle318 materializes the cycle-318 acceptance criteria for the two
// committed top_n tasks (triage-report.md "## top_n"):
//
//	cmd-loop-fail-breaker-isolation — TestRunLoop_FailVerdictBreaks inherits the
//	    operator's ambient EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3, so the loop never
//	    trips the stop-on-first-fail breaker and exits via max_cycles (rc=3)
//	    instead of fail (rc=2). The fix isolates the test from the ambient var
//	    (a t.Setenv call). After the fix the test — and the whole cmd/evolve
//	    suite — is green even when the var is set.
//
//	ledger-seal-io-coverage — add targeted tests for the low-coverage seal I/O
//	    helpers (writeSegment 50.0%, rewriteLive 52.2%, readSegment 71.4%,
//	    linesEqual 66.7%; package total 82.4%), lifting internal/adapters/ledger
//	    statement coverage to the committed >= 85.0% floor while the existing
//	    TestSeal* contract stays green.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). They run the REAL test
// suites in a subprocess and assert on the measured exit code / coverage
// percentage — no source-grep gaming:
//
//   - Task 1 predicates run cmd/evolve under an INJECTED
//     EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3 (cmd.Env), so they require the
//     isolation fix REGARDLESS of the host's ambient env: pre-fix the loop
//     continues past both FAIL cycles and exits rc=3 → subprocess non-zero →
//     RED; post-fix the test's own isolation overrides the injected value →
//     rc=2 → exit 0 → GREEN. The predicate encodes INTENT (the test isolates
//     from the ambient var) not a specific implementation line, so any valid
//     isolation mechanism satisfies it. A magic string cannot make a failing
//     loop exit rc=2.
//   - Task 2 coverage gates RUN the real ledger suite under -coverprofile and
//     assert on `go tool cover -func`. A magic string in a source file cannot
//     move the number — only Builder's new tests, which actually call the
//     helpers, can. An EMPTY repo (no tests) yields 0% and fails every floor,
//     so they are anti-no-op by construction. coverFuncOutput Fatals (RED) if
//     the suite does not compile or any test FAILs, so every coverage gate also
//     folds in the no-regression axis.
//
// AC map (1:1 with the scout-report.md "Acceptance Criteria Summary" — 4 ACs —
// plus one adversarial negative predicate):
//
//	cmd-loop-fail-breaker-isolation
//	  AC1 full cmd/evolve suite exit 0          → C318_002
//	  AC2 TestRunLoop_FailVerdictBreaks exit 0  → C318_001
//	ledger-seal-io-coverage
//	  AC1 ledger coverage >= 85.0%              → C318_003
//	      (negative/edge axis of AC1)           → C318_005 (linesEqual false branch)
//	  AC2 TestSeal* suite exit 0                → C318_004
//
// Floor binding (R9.3): internal/adapters/ledger is a committed top_n task this
// cycle, so the coverage floor (C318_003/C318_005) binds a committed package.
// The triage-DEFERRED items (bridge-*, preserve-worktree-on-verdict-fail) get
// ZERO predicates here — authoring a floor on a deferred task would starve the
// committed ones (cycle-280 lesson).
package cycle318

import (
	"errors"
	"os"
	"os/exec"
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
	cmdEvolvePkg = "./cmd/evolve/..."
	ledgerPkg    = "./internal/adapters/ledger/..."
)

// tail returns the last n lines of s (keeps RED failure messages readable).
func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// runGoTest runs `go test -C <goDir> <args...>` with extraEnv appended to the
// inherited environment and returns the combined stdout+stderr and the exit
// code. Fatals only when the toolchain itself fails to launch (not on a
// non-zero test exit, which callers assert on). Using exec directly (rather
// than acsassert.SubprocessOutput) is deliberate: it is the only way to inject
// EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS into the child so the Task 1 predicates
// require the isolation fix independent of the host's ambient env.
func runGoTest(t *testing.T, extraEnv []string, args ...string) (combined string, code int) {
	t.Helper()
	full := append([]string{"test", "-C", goDir(t)}, args...)
	cmd := exec.Command("go", full...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err == nil {
		return buf.String(), 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return buf.String(), ee.ExitCode()
	}
	t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, tail(buf.String(), 30))
	return "", -1
}

// coverTotalRe matches the trailing `total:  (statements)  NN.N%` line.
var coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)

// coverFuncOutput runs the REAL ledger suite under -coverprofile and returns
// the `go tool cover -func` report. Fatals (RED) if the suite does not compile
// or a test FAILs — a coverage number is only meaningful over a green suite, so
// this folds in the no-regression axis.
func coverFuncOutput(t *testing.T) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "c.cover")
	combined, code := runGoTest(t, nil, "-count=1", "-coverprofile="+profile, ledgerPkg)
	if code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d) — add the new seal I/O tests / fix regressions:\n%s",
			ledgerPkg, code, tail(combined, 40))
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

// =============== cmd-loop-fail-breaker-isolation predicates ===============

// --- C318_001 (AC2): TestRunLoop_FailVerdictBreaks isolates from the ambient
// EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS var --------------------------------------
//
// The defining behavioral predicate. We INJECT EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3
// into the child (cmd.Env) — the exact ambient condition that breaks the test
// today — and require the targeted test to pass anyway. Pre-fix: the test does
// not isolate, resolveMaxConsecutiveFails() returns 3, the loop continues past
// both FAIL cycles and exits stop_reason=max_cycles (rc=3), the in-test
// assertion `rc != 2` fires → subprocess non-zero → RED. Post-fix: the test's
// own isolation overrides the injected value → rc=2 → exit 0 → GREEN. This pins
// INTENT (the test is hermetic w.r.t. the ambient var), so any valid isolation
// mechanism satisfies it; no source string can make a failing loop exit rc=2.
func TestC318_001_FailVerdictBreakerIsolatesFromAmbientEnv(t *testing.T) {
	combined, code := runGoTest(t, []string{"EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3"},
		"-run", "TestRunLoop_FailVerdictBreaks", "-count=1", "-short", cmdEvolvePkg)
	if code != 0 {
		t.Errorf("RED: TestRunLoop_FailVerdictBreaks fails when EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3 "+
			"is present in the environment (exit=%d) — isolate it (e.g. "+
			"t.Setenv(\"EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS\", \"1\")) so the breaker trips on the "+
			"first FAIL and the loop exits rc=2:\n%s", code, tail(combined, 25))
	}
}

// --- C318_002 (AC1): the full cmd/evolve suite is green under the ambient var --
//
// Regression axis. Runs the WHOLE cmd/evolve suite under the same injected
// EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3. Today only TestRunLoop_FailVerdictBreaks
// fails under that condition (scout health check), so the suite is RED until
// the isolation fix lands; afterwards it is green even with the var set.
func TestC318_002_CmdEvolveSuiteGreenUnderAmbientVar(t *testing.T) {
	combined, code := runGoTest(t, []string{"EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3"},
		"-count=1", "-short", cmdEvolvePkg)
	if code != 0 {
		t.Errorf("RED: full cmd/evolve suite fails when EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3 is set "+
			"(exit=%d) — the fail-breaker isolation fix must make it green:\n%s", code, tail(combined, 30))
	}
}

// =================== ledger-seal-io-coverage predicates ===================

// --- C318_003 (AC1): internal/adapters/ledger total coverage >= 85.0% --------
//
// Behavioral coverage-floor over the real ledger suite. RED baseline: 82.4%.
// GREEN requires Builder's new tests for the low-coverage seal I/O helpers
// (writeSegment, rewriteLive, readSegment, linesEqual). The shared green-suite
// gate in coverFuncOutput Fatals if any existing test regresses, so this folds
// in the no-regression axis too. Floor pinned at the triage-committed 85.0%
// (scout-report Acceptance Criteria) — Builder may deliver higher.
func TestC318_003_LedgerCoverageFloor(t *testing.T) {
	const floor = 85.0
	out := coverFuncOutput(t)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/adapters/ledger coverage %.1f%% < %.1f%% floor — add seal I/O tests "+
			"(writeSegment/rewriteLive/readSegment/linesEqual). Baseline 82.4%%.", pct, floor)
	}
}

// --- C318_004 (AC2): the TestSeal* contract suite stays green -----------------
//
// Regression axis for the seal contract (chain-preservation, resume-after-crash,
// tamper-detection). Pre-existing GREEN today — Builder must KEEP it green while
// adding the new I/O tests. Uses a distinct command verb (-run TestSeal) from
// the coverage gate for lexical diversity.
func TestC318_004_SealContractSuiteGreen(t *testing.T) {
	combined, code := runGoTest(t, nil, "-run", "TestSeal", "-count=1", "-short", ledgerPkg)
	if code != 0 {
		t.Errorf("RED: the TestSeal* contract suite fails (exit=%d) — seal I/O tests must not break "+
			"the chain-preservation / resume-after-crash contract:\n%s", code, tail(combined, 30))
	}
}

// --- C318_005 (AC1 negative axis): linesEqual's FALSE branch is exercised -----
//
// Adversarial NEGATIVE predicate (adversarial-testing SKILL §6). linesEqual is a
// 3-statement helper at 66.7% today — exactly one branch (the unequal case) is
// dark. A happy-path-only test suite can reach an aggregate coverage number
// without ever asserting the rejection behavior; this gate forces a test that
// drives the FALSE branch (a length mismatch and/or an element mismatch returns
// false). 100% is achievable for a 3-statement function, so this is a tight but
// reachable floor that pins the negative dimension the eval requires.
func TestC318_005_LinesEqualFalseBranchCovered(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "linesEqual"); pct < floor {
		t.Errorf("RED: linesEqual coverage %.1f%% < 100%% — add a NEGATIVE test driving the "+
			"unequal-elements (and/or length-mismatch) FALSE branch. Baseline 66.7%%.", pct)
	}
}
