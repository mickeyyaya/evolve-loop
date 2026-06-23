//go:build acs

// Package cycle320 materializes the cycle-320 acceptance criteria for the one
// committed top_n task (triage-report.md "## top_n"):
//
//	phasecoherence-coverage-gaps — raise go/internal/phasecoherence statement
//	    coverage from the 85.2% baseline to the committed >= 90.0% floor by
//	    exercising the dark branches of canonicalRole (42.9% — only the default
//	    lower-casing arm is hit; the seven exact-match arms scout/builder/build/
//	    auditor/audit/intent/memo are dark) and dispatchNone (75.0% — the happy
//	    `dispatch: none` arm is reached via TestCoherence_DispatchNonePersonaExempt
//	    but the error / parse-fail / non-"none" arms are dark), while every
//	    existing phasecoherence test stays green.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). They RUN the real
// phasecoherence test suite in a subprocess under -coverprofile and assert on
// the measured `go tool cover -func` percentages — there is no source-grep:
//
//   - A magic string in a source file cannot move a coverage number. Only
//     Builder's new tests, which actually CALL canonicalRole / dispatchNone with
//     the previously-dark inputs, can.
//   - An EMPTY repo (no tests) yields 0% and fails every floor, so the gates are
//     anti-no-op by construction.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, so every coverage gate folds in the no-regression axis (AC4).
//   - The per-function 100.0% floors are the ADVERSARIAL axis (adversarial-testing
//     SKILL §6): canonicalRole and dispatchNone are small enough that 100% is
//     reachable, so the floor forces the NEGATIVE/edge cases — dispatchNone's
//     error & non-"none" rejection arms, every exact-match arm of canonicalRole —
//     not just an aggregate number a happy-path-only suite could reach.
//
// AC map (1:1 with the scout-report.md "Acceptance Criteria" list — 4 ACs):
//
//	phasecoherence-coverage-gaps
//	  AC1 package coverage >= 90.0%                      → C320_001
//	  AC2 canonicalRole exact-match branches exercised   → C320_002 (func 100%)
//	  AC3 dispatchNone reached for dispatch:none + arms  → C320_003 (func 100%)
//	  AC4 no regression (existing tests stay green)      → C320_004
//
// Floor binding (R9.3): internal/phasecoherence is the committed top_n task this
// cycle, so the coverage floor (C320_001/002/003) binds a committed package. The
// scout-DEFERRED items (cmd/evolve, modelcatalog Write) get ZERO predicates here
// — authoring a floor on a deferred task would starve the committed one
// (cycle-280 lesson).
package cycle320

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

const phasecoherencePkg = "./internal/phasecoherence/..."

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

// coverFuncOutput runs the REAL phasecoherence suite under -coverprofile and
// returns the `go tool cover -func` report. Fatals (RED) if the suite does not
// compile or a test FAILs — a coverage number is only meaningful over a green
// suite, so this folds in the no-regression axis (AC4).
func coverFuncOutput(t *testing.T) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "c.cover")
	combined, code := runGoTest(t, "-count=1", "-coverprofile="+profile, phasecoherencePkg)
	if code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d) — add the canonicalRole / dispatchNone "+
			"branch tests / fix regressions:\n%s", phasecoherencePkg, code, tail(combined, 40))
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

// ================ phasecoherence-coverage-gaps predicates =================

// --- C320_001 (AC1): go/internal/phasecoherence total coverage >= 90.0% ------
//
// Behavioral coverage-floor over the real phasecoherence suite. RED baseline:
// 85.2%. GREEN requires Builder's new canonicalRole exact-match + dispatchNone
// branch tests. The shared green-suite gate in coverFuncOutput Fatals if any
// existing test regresses, so this folds in the no-regression axis too. Floor
// pinned at the triage-committed 90.0% (scout-report AC1) — Builder may deliver
// higher.
func TestC320_001_PhasecoherenceCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/phasecoherence coverage %.1f%% < %.1f%% floor — exercise the dark "+
			"canonicalRole exact-match arms and dispatchNone branches. Baseline 85.2%%.", pct, floor)
	}
}

// --- C320_002 (AC2): canonicalRole exact-match branches all exercised ---------
//
// Behavioral + anti-gaming. canonicalRole is a pure switch at 42.9% today — only
// the `default` lower-casing arm is hit (the existing amplification test passes
// only mixed-case / unknown inputs). The seven exact-match arms (scout, builder,
// build, auditor, audit, intent, memo) are dark. 100% is reachable for a pure
// switch, so this floor REQUIRES a test that drives every exact-match arm — the
// scout AC's "all exercised" condition. A magic string cannot move func coverage;
// only real calls with the lower-case canonical inputs can.
func TestC320_002_CanonicalRoleBranchesCovered(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "canonicalRole"); pct < floor {
		t.Errorf("RED: canonicalRole coverage %.1f%% < 100%% — add table rows for the exact-match "+
			"inputs (scout, builder, build, auditor, audit, intent, memo) so every switch arm runs. "+
			"Baseline 42.9%% (only the default arm).", pct)
	}
}

// --- C320_003 (AC3): dispatchNone reached for dispatch:none + reject arms ------
//
// Behavioral + ADVERSARIAL negative axis. dispatchNone is at 75.0% today: the
// happy `dispatch: none → true` arm is reached transitively via
// TestCoherence_DispatchNonePersonaExempt, but the rejection arms are dark —
// personaContents error → false, ParseFrontmatter error / nil frontmatter →
// false, and a present-but-non-"none" dispatch value → false. 100% is reachable
// for this 6-statement helper, so the floor forces a NEGATIVE test driving those
// reject arms in addition to the dispatch:none-returns-true case the scout AC3
// names. (dispatchNone is package-internal; Builder can call it directly from a
// `package phasecoherence` test, as the existing canonicalRole test does.)
func TestC320_003_DispatchNoneBranchesCovered(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "dispatchNone"); pct < floor {
		t.Errorf("RED: dispatchNone coverage %.1f%% < 100%% — add a test that returns true for a "+
			"`dispatch: none` persona AND drives the reject arms (unreadable persona → false, "+
			"unparsable/absent frontmatter → false, non-\"none\" dispatch value → false). "+
			"Baseline 75.0%%.", pct)
	}
}

// --- C320_004 (AC4): the existing phasecoherence contract suite stays green ----
//
// Regression axis. Runs the pre-existing TestCoherence* + TestCanonicalRole*
// contract suite (clean-pair, disallowed/undeclared drift, normalization,
// unpaired/dispatch-none exemption, override substitution). Pre-existing GREEN
// today — Builder must KEEP it green while adding the new branch tests. Uses a
// distinct -run command verb from the coverage gate for lexical diversity.
func TestC320_004_ExistingContractSuiteGreen(t *testing.T) {
	combined, code := runGoTest(t, "-run", "TestCoherence|TestCanonicalRole", "-count=1", phasecoherencePkg)
	if code != 0 {
		t.Errorf("RED: the existing phasecoherence contract suite fails (exit=%d) — the new "+
			"coverage tests must not break the coherence / canonicalRole contract:\n%s",
			code, tail(combined, 30))
	}
}
