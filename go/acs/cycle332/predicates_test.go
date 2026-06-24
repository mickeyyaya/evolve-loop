//go:build acs

// Package cycle332 materializes the cycle-332 acceptance criteria for the TWO
// committed top_n tasks (triage-report.md "## top_n"), both lifting test
// coverage of isolated dark error branches:
//
//	evalgate-error-branch-coverage — push internal/evalgate from 91.06% to the
//	    scout's >= 93.0% AC by covering FOUR dark statements:
//	      - cycleNumFromWorkspace nil-match `return 0`  (floorbinding.go:95)
//	      - cycleNumFromWorkspace Atoi-error `return 0`  (floorbinding.go:99)
//	      - fencedAfterHeading no-trailing-newline else  (slugs.go:128)
//	      - NewReviewer logf closure body, run via Review (reviewer.go:40)
//
//	releasepipeline-default-verify-error-branches — lift defaultReleaseVerify
//	    from 0.0% by covering its two early-exit guards (relative-repoRoot
//	    reject, missing-binary-on-disk) and add a DIRECT defaultShip
//	    binary-not-found characterization test.
//
// These predicates are BEHAVIORAL (cycle-85 lesson) — there is NO load-bearing
// source-grep:
//
//   - The coverage gates RUN the real package suite under -coverprofile and
//     assert on the measured `go tool cover -func` percentages. A magic string
//     cannot move a coverage number — only Builder's new tests can — so they are
//     anti-no-op by construction; an EMPTY repo yields 0% and fails every floor.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, folding the no-regression axis into every coverage gate.
//   - The named-test PASS gates RUN the suite filtered to the committed test
//     functions and require each `--- PASS:` line; a no-op -run match prints
//     "no tests to run" with ZERO PASS lines (RED today), so they fail until
//     Builder's real tests exist and pass.
//   - The per-function anchors (cycleNumFromWorkspace / fencedAfterHeading /
//     NewReviewer / defaultReleaseVerify) pin the % movement to the TARGET dark
//     branches, not to incidental coverage elsewhere.
//
// ⚠ SCOUT-CORRECTION NOTE (Core Rule 3 — surfaced conflict; cycle-280/331 lesson
// — never pin an UNREACHABLE floor, but never lower a REACHABLE bar either):
// the scout asserted the cycleNumFromWorkspace Atoi-error branch is
// "unreachable" (regex guarantees digits). That is WRONG: strconv.Atoi of an
// all-digit string still fails on integer OVERFLOW (e.g. a 25-nine cycle
// number > int64 max → *NumError ErrRange). The correction matters because the
// scout's own >= 93.0% AC is NOT reachable WITHOUT covering that branch —
// statement math: 163/179 baseline + the 3 "reachable" branches (nil-match,
// no-newline, logf) = 166/179 = 92.74% < 93.0%. Covering the Atoi-overflow
// branch too gives 167/179 = 93.30%, the FIRST value that clears 93.0%. So the
// 93.0% AC is preserved (not adjusted) and the cycleNumFromWorkspace floor is
// pinned at 100% to FORCE both return-0 branches, including the overflow test
// the scout missed. test-report.md records this disposition.
//
// AC map (1:1 with the committed-task acceptance criteria):
//
//	evalgate-error-branch-coverage
//	  AC-E1 cycleNumFromWorkspace nil-match + Atoi-overflow covered → C332_001 (== 100%)
//	  AC-E2 fencedAfterHeading no-newline-else covered              → C332_002 (== 100%)
//	  AC-E3 NewReviewer logf closure body exercised via Review      → C332_003 (== 100%)
//	  AC-E4 evalgate package coverage lifts to the scout floor       → C332_004 (>= 93.0%)
//	releasepipeline-default-verify-error-branches
//	  AC-R1 the two defaultReleaseVerify guard tests exist & PASS   → C332_005 (named-test gate)
//	  AC-R2 defaultReleaseVerify coverage lifts off 0.0%            → C332_006 (>= 10.0%)
//	  AC-R3 defaultShip binary-not-found direct test exists & PASS  → C332_007 (named-test gate)
//
// Floor binding (R9.3): internal/evalgate and internal/releasepipeline are the
// SOLE packages the two committed top_n tasks target, so every floor/gate binds
// committed work. No triage-DEFERRED item gets a predicate here — a floor on a
// deferred task would starve the committed ones (cycle-280 lesson).
package cycle332

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evalgatePkg = "./internal/evalgate/"
	releasePkg  = "./internal/releasepipeline/"
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

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

// totalCoverage parses the trailing `total:\t(statements)\t<pct>%` row.
func totalCoverage(t *testing.T, out string) float64 {
	t.Helper()
	re := regexp.MustCompile(`(?m)^total:\s+\(statements\)\s+([0-9.]+)%`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("could not find total coverage row:\n%s", tail(out, 10))
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("unparsable total coverage %q: %v", m[1], err)
	}
	return pct
}

// requirePassLines runs pkg's suite filtered to the given -run regex with -v and
// requires a clean exit PLUS a `--- PASS: <name>` line for every name. This is
// the anti-no-op binding for a deliverable that is named test functions: `go
// test -run` over a NON-matching pattern exits 0 with "no tests to run" and ZERO
// PASS lines (verified RED today), so requiring the explicit PASS lines makes the
// gate fail until Builder's real tests exist and pass. A renamed/removed/failing
// test drops its PASS line and re-reddens the gate; a source-grep or empty repo
// can never satisfy it.
func requirePassLines(t *testing.T, pkg, runRegex string, names []string) {
	t.Helper()
	combined, code := runGoTest(t, "-v", "-count=1", "-run", runRegex, pkg)
	if code != 0 {
		t.Errorf("RED: %s suite fails (exit=%d) for -run %q — the committed tests "+
			"must exist and PASS:\n%s", pkg, code, runRegex, tail(combined, 40))
		return
	}
	for _, n := range names {
		passLine := "--- PASS: " + n
		if !strings.Contains(combined, passLine) {
			t.Errorf("RED: %q not present — %s is missing, renamed, or did not PASS "+
				"(a no-op -run match prints \"no tests to run\" and zero PASS lines):\n%s",
				passLine, n, tail(combined, 40))
		}
	}
}

// ================ evalgate-error-branch-coverage predicates ==================

// --- C332_001 (AC-E1): cycleNumFromWorkspace == 100% -------------------------
//
// Behavioral + branch anchor. cycleNumFromWorkspace is 71.4% today (5/7
// statements): BOTH `return 0` arms are dark — the nil-match arm
// (floorbinding.go:95, a non-`cycle-<N>` basename) and the strconv.Atoi-error
// arm (floorbinding.go:99). Reaching 100% REQUIRES a nil-match test (e.g.
// "workspace" or "cycle-300/artifacts") AND an Atoi-overflow test (a basename
// like "cycle-99999999999999999999999999" whose digits overflow int64 → Atoi
// ErrRange → return 0). The scout called the Atoi arm "unreachable"; it is
// reachable via overflow and MUST be covered or the package gate cannot reach
// 93.0% (see the scout-correction note). 7/7 = 100% is the exact ceiling.
func TestC332_001_CycleNumFromWorkspaceBothReturnZero(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t, evalgatePkg)
	if pct := funcCoverage(t, out, "cycleNumFromWorkspace"); pct < floor {
		t.Errorf("RED: evalgate.cycleNumFromWorkspace coverage %.1f%% < %.1f%% — cover BOTH return-0 arms: "+
			"a non-cycle basename (nil-match) AND an overflowing all-digit basename "+
			"(cycle-99999999999999999999999999 → strconv.Atoi ErrRange). Baseline 71.4%%; the Atoi arm is "+
			"REACHABLE via overflow (scout's 'unreachable' is wrong) and is required to clear the 93%% floor.",
			pct, floor)
	}
}

// --- C332_002 (AC-E2): fencedAfterHeading == 100% ----------------------------
//
// Behavioral + branch anchor. fencedAfterHeading is 80.0% today (4/5
// statements): the `else { return "", false }` arm (slugs.go:128) — taken when
// the opening fence line has NO trailing newline (strings.IndexByte returns -1)
// — is dark. Reaching 100% REQUIRES a test whose report ends with an opening
// "\n```" fence and no subsequent newline, asserting ("", false). 5/5 = 100%.
func TestC332_002_FencedAfterHeadingNoNewlineElse(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t, evalgatePkg)
	if pct := funcCoverage(t, out, "fencedAfterHeading"); pct < floor {
		t.Errorf("RED: evalgate.fencedAfterHeading coverage %.1f%% < %.1f%% — add a test where the opening "+
			"fence line lacks a trailing newline (report ends at \"\\n```\") and assert it returns (\"\", false). "+
			"Baseline 80.0%%.", pct, floor)
	}
}

// --- C332_003 (AC-E3): NewReviewer == 100% -----------------------------------
//
// Behavioral + branch anchor. NewReviewer is 50.0% today (1/2 statements): the
// logf closure body (reviewer.go:40, `fmt.Fprintf(os.Stderr, ...)`) is dark
// because no test drives Review() to a real violation. Reaching 100% REQUIRES a
// test that builds NewReviewer(StageShadow) and calls Review with an input that
// trips a gate (e.g. materialization: a scout deliverable selecting a slug whose
// .evolve/evals/<slug>.md is absent), so the gate returns reason != "" and
// r.logf executes. 2/2 = 100%. (StageShadow keeps it approve-only, so the gate
// logs without aborting — exactly what exercises the closure.)
func TestC332_003_NewReviewerLogfBodyExercised(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t, evalgatePkg)
	if pct := funcCoverage(t, out, "NewReviewer"); pct < floor {
		t.Errorf("RED: evalgate.NewReviewer coverage %.1f%% < %.1f%% — drive Review() to a real violation "+
			"(NewReviewer(config.StageShadow), then Review with an input that trips a gate) so the logf "+
			"closure body runs. Baseline 50.0%%.", pct, floor)
	}
}

// --- C332_004 (AC-E4): evalgate package coverage >= 93.0% --------------------
//
// Behavioral + headline anti-no-op gate. internal/evalgate is 91.06% today
// (163/179 statements). Covering the four dark statements pinned by C332_001..3
// (nil-match, Atoi-overflow, no-newline-else, logf body) yields 167/179 =
// 93.30%, the first value clearing the scout's 93.0% AC — so this gate is
// guaranteed GREEN once the per-function gates pass, and RED until they do. The
// margin is exactly one statement (166/179 = 92.74% fails), which is WHY the
// Atoi-overflow branch is mandatory; a no-op cannot move package coverage at all.
func TestC332_004_EvalgatePackageCoverageFloor(t *testing.T) {
	const floor = 93.0
	out := coverFuncOutput(t, evalgatePkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/evalgate coverage %.2f%% < %.1f%% — the four dark branch tests "+
			"(cycleNumFromWorkspace x2, fencedAfterHeading, NewReviewer logf) must all land. Baseline 91.06%%; "+
			"all FOUR statements are required (three alone = 92.74%% < 93.0%%).", pct, floor)
	}
}

// ========= releasepipeline-default-verify-error-branches predicates ==========

// --- C332_005 (AC-R1): the two defaultReleaseVerify guard tests exist & PASS -
//
// Behavioral named-test gate. RED today: neither test exists, so the -run
// pattern matches nothing ("no tests to run", zero PASS lines). GREEN only when
// Builder adds (in default_release_verify_test.go, package releasepipeline so it
// can call the private func):
//   - TestDefaultReleaseVerify_RelativeRepoRoot — defaultReleaseVerify("rel/path",
//     ...) returns a non-nil "must be absolute" error (covers floorbinding guard
//     releasepipeline.go:697).
//   - TestDefaultReleaseVerify_MissingBinaryOnDisk — defaultReleaseVerify(
//     t.TempDir() [absolute, no go/evolve], ...) returns a "tracked binary
//     missing on disk" error (covers releasepipeline.go:703-705).
func TestC332_005_DefaultReleaseVerifyGuardTestsPass(t *testing.T) {
	requirePassLines(t, releasePkg,
		"TestDefaultReleaseVerify_RelativeRepoRoot|TestDefaultReleaseVerify_MissingBinaryOnDisk",
		[]string{
			"TestDefaultReleaseVerify_RelativeRepoRoot",
			"TestDefaultReleaseVerify_MissingBinaryOnDisk",
		})
}

// --- C332_006 (AC-R2): defaultReleaseVerify coverage >= 10.0% ----------------
//
// Behavioral + branch anchor + headline. defaultReleaseVerify is 0.0% today.
// The two guard tests of C332_005 cover the entry block + relative-path return
// + binAbs/ReadFile block + missing-binary return ≈ 7 statements of ~36 ≈ 19.4%.
// The floor is the ACHIEVABLE-and-strict 10.0% (>= the scout's literal ">0.0%"
// AC, with ~2-statement margin under the ~19% the two committed tests reach);
// the cat-file "not committed", SHA-mismatch, state.json re-pin, version, and
// tag arms need a real git repo + committed blob (scout-deferred, BA2), so they
// stay dark — 10% is calibrated to what the committed tests deliver, not to the
// full function. A no-op cannot move coverage off 0%.
func TestC332_006_DefaultReleaseVerifyCoverageOffZero(t *testing.T) {
	const floor = 10.0
	out := coverFuncOutput(t, releasePkg)
	if pct := funcCoverage(t, out, "defaultReleaseVerify"); pct < floor {
		t.Errorf("RED: releasepipeline.defaultReleaseVerify coverage %.1f%% < %.1f%% — the relative-repoRoot "+
			"and missing-binary-on-disk guard tests must exercise the early-exit branches. Baseline 0.0%%; "+
			"the cat-file/SHA/re-pin/version/tag arms (need a real git repo) are scout-deferred.", pct, floor)
	}
}

// --- C332_007 (AC-R3): defaultShip binary-not-found direct test exists & PASS -
//
// Behavioral named-test gate. NOTE (Core Rule 12 — fail loudly): defaultShip's
// binary-not-found arm (releasepipeline.go:618-621) is ALREADY incidentally
// covered by TestRun_NilShipOverriddenByDefault (which clears EVOLVE_GO_BIN+PATH
// and drives Run → defaultShip). The committed deliverable is therefore a
// DIRECT characterization test that calls defaultShip itself with no resolvable
// binary and asserts the "evolve binary not found" error — explicit, not
// incidental. RED today: TestDefaultShip_BinaryNotFound does not exist. GREEN
// when Builder adds it (clear EVOLVE_GO_BIN + PATH, t.TempDir() repoRoot with no
// go/{bin/,}evolve, assert a non-nil error mentioning "binary not found").
func TestC332_007_DefaultShipBinaryNotFoundDirectTestPass(t *testing.T) {
	requirePassLines(t, releasePkg,
		"TestDefaultShip_BinaryNotFound",
		[]string{"TestDefaultShip_BinaryNotFound"})
}
