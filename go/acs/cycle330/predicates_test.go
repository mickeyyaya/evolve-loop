//go:build acs

// Package cycle330 materializes the cycle-330 acceptance criteria for the THREE
// committed top_n tasks (triage-report.md "## top_n"):
//
//	bridge-ratelimit-matches-agent-content — close the remaining residual of the
//	    soak-#4 cycle-314 content-vs-chrome false positive: a BARE unified-diff
//	    line ("+content"/"-content" WITHOUT a leading line number) carrying a
//	    quoted rate-limit banner must NOT drive the codex-tmux escalate rule when
//	    the pane is IDLE (the numbered-diff strip and the ADR-0047 busy idle-gate
//	    both miss it), while a genuine banner (CLI chrome, never diff-prefixed)
//	    still escalates. Scoped per triage to the bare-diff-exclusion sub-fix +
//	    TDD pin (footer/status-chrome scoping + persistence gating stay deferred).
//
//	interaction-error-branch-coverage — exercise the three uncovered error/edge
//	    branches in go/internal/interaction: ledgerPath's empty-phase "unknown"
//	    fallback (66.7% baseline), neutralize's second rune-cap on a multi-line
//	    Digest (83.3%), and appendLedgerLine's swallowed os.OpenFile error
//	    (77.8%). appendLedgerLine's json.Marshal error arm is unreachable (Outcome
//	    always marshals), so ~88.9% is its practical ceiling — its floor is 85,
//	    not 100.
//
//	phasecoherence-check-nil-guard-coverage — exercise the four uncovered guard/
//	    skip branches in phasecoherence.Check (81.4% baseline): the nil-AgentsFS
//	    error (distinct from empty-FS), the directory-entry skip (entry.IsDir),
//	    the nil-frontmatter skip (fm == nil), and the non-[]string toolsVal skip.
//
// These predicates are BEHAVIORAL (cycle-85 lesson) — there is no load-bearing
// source-grep:
//
//   - The bridge gate RUNS the real decideAutoRespond truth-table suite in a
//     subprocess and asserts both the exit code and that the new bare-diff
//     PASS line is present; a magic string in a source file cannot satisfy it,
//     and an EMPTY repo (no bridge tests) cannot produce the PASS line.
//   - The coverage gates RUN the real package suites under -coverprofile and
//     assert on the measured `go tool cover -func` percentages. A magic string
//     cannot move a coverage number — only Builder's new tests can — so the
//     coverage gates are anti-no-op by construction. An EMPTY repo yields 0% and
//     fails every floor.
//   - coverFuncOutput Fatals (RED) if a suite does not compile or any test
//     FAILs, so every coverage gate folds in the no-regression axis; a dedicated
//     -race gate per package adds the data-race axis with a distinct command verb.
//   - The per-function anchors (ledgerPath / neutralize / appendLedgerLine /
//     Check) pin the % movement to the TARGET dark branches, not to incidental
//     coverage elsewhere in the package.
//
// AC map (1:1 with the committed-task acceptance criteria; the "no production
// code beyond the scoped fix" / "tests not modified" criteria are dispositioned
// manual+checklist for the Auditor in test-report.md, not as a fragile git-diff
// predicate whose result depends on phase-commit timing):
//
//	bridge-ratelimit-matches-agent-content
//	  AC-A1 bare diff line (idle) → no escalate    } both → C330_001
//	  AC-A2 real banner → still escalates          }
//	interaction-error-branch-coverage
//	  AC-B1 ledgerPath empty-phase fallback covered  → C330_002 (ledgerPath >= 90%)
//	  AC-B2 neutralize multi-line rune-cap covered   → C330_003 (neutralize >= 90%)
//	  AC-B3 appendLedgerLine OpenFile error covered  → C330_004 (appendLedgerLine >= 85%)
//	  AC-B4 interaction suite stays green (-race)    → C330_005
//	phasecoherence-check-nil-guard-coverage
//	  AC-C1 four nil/skip branches covered           → C330_006 (Check >= 88%)
//	  AC-C2 phasecoherence suite stays green (-race)  → C330_007
//
// Floor binding (R9.3): internal/bridge, internal/interaction, and
// internal/phasecoherence are the THREE committed top_n tasks this cycle, so
// every floor/gate binds committed work. The triage-DEFERRED item
// evalgate-floorbinding-workspace-coverage gets ZERO predicates here — a floor
// on a deferred task would starve the committed ones (cycle-280 lesson).
package cycle330

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
	interactionPkg = "./internal/interaction/..."
	coherencePkg   = "./internal/phasecoherence/..."
	bridgePkg      = "./internal/bridge/"
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

// ============= bridge-ratelimit-matches-agent-content predicate ==============

// --- C330_001 (AC-A1 + AC-A2): bare unified-diff lines do not escalate -------
//
// Behavioral. Runs the real decideAutoRespond truth-table suite (TestDecideAuto
// Respond*) in the bridge package and requires BOTH a clean exit AND the new
// bare-diff PASS line. RED today: the bare "+/-content" diff lines on an idle
// pane still match the escalate rate_limit rule (numbered-diff strip + busy
// idle-gate both miss them), so TestDecideAutoRespond_BareDiffLineNotChrome
// FAILs and the gate is RED until Builder strips bare diff lines before
// escalate matching. The explicit PASS-line check is the anti-no-op binding:
// it cannot be satisfied by a source grep or by deleting the test (a renamed/
// removed test drops the PASS line), and an empty repo produces no PASS line.
func TestC330_001_BridgeBareDiffNoEscalate(t *testing.T) {
	combined, code := runGoTest(t, "-v", "-count=1", "-run", "TestDecideAutoRespond", bridgePkg)
	if code != 0 {
		t.Errorf("RED: decideAutoRespond suite fails (exit=%d) — a bare unified-diff line "+
			"(\"+content\"/\"-content\", no leading line number) carrying banner text on an IDLE pane "+
			"must be stripped before escalate-pattern matching; a real banner must still escalate:\n%s",
			code, tail(combined, 40))
	}
	const passLine = "--- PASS: TestDecideAutoRespond_BareDiffLineNotChrome"
	if !strings.Contains(combined, passLine) {
		t.Errorf("RED: %q not present — the bare-diff exclusion test did not PASS (failing, renamed, or removed):\n%s",
			passLine, tail(combined, 40))
	}
}

// ============== interaction-error-branch-coverage predicates =================

// --- C330_002 (AC-B1): interaction.ledgerPath coverage >= 90% ----------------
//
// Behavioral + branch anchor. ledgerPath is 66.7% today (2/3 statements): the
// empty-phase `phase = "unknown"` fallback is dark. Reaching 90% REQUIRES a test
// that calls Record/appendLedgerLine with an empty Phase. A magic string cannot
// move this number — only the new test can.
func TestC330_002_LedgerPathEmptyPhaseCovered(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t, interactionPkg)
	if pct := funcCoverage(t, out, "ledgerPath"); pct < floor {
		t.Errorf("RED: interaction.ledgerPath coverage %.1f%% < %.1f%% — add a test driving the "+
			"empty-phase \"unknown\" fallback. Baseline 66.7%%.", pct, floor)
	}
}

// --- C330_003 (AC-B2): interaction.neutralize coverage >= 90% ----------------
//
// Behavioral + branch anchor. neutralize is 83.3% today (5/6): the second
// rune-cap `if len(r) > payloadMaxChars { d = string(r[:payloadMaxChars]) }` on
// a joined multi-line Digest is dark. Reaching 90% REQUIRES a multi-line payload
// whose joined Digest exceeds payloadMaxChars (200 runes).
func TestC330_003_NeutralizeMultiLineRuneCapCovered(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t, interactionPkg)
	if pct := funcCoverage(t, out, "neutralize"); pct < floor {
		t.Errorf("RED: interaction.neutralize coverage %.1f%% < %.1f%% — add a multi-line payload "+
			"(>200 joined runes) that triggers the second rune-cap. Baseline 83.3%%.", pct, floor)
	}
}

// --- C330_004 (AC-B3): interaction.appendLedgerLine coverage >= 85% ----------
//
// Behavioral + negative anchor. appendLedgerLine is 77.8% today (7/9): the
// swallowed os.OpenFile error arm is dark. Reaching 85% REQUIRES a test that
// makes OpenFile fail (e.g. a workspace path whose phase-file parent is a
// regular file, not a directory). The json.Marshal error arm is unreachable
// (Outcome always marshals), so ~88.9% (8/9) is the practical ceiling — the
// floor is 85, not 100, on purpose.
func TestC330_004_AppendLedgerLineOpenFileErrorCovered(t *testing.T) {
	const floor = 85.0
	out := coverFuncOutput(t, interactionPkg)
	if pct := funcCoverage(t, out, "appendLedgerLine"); pct < floor {
		t.Errorf("RED: interaction.appendLedgerLine coverage %.1f%% < %.1f%% — add a test that drives "+
			"os.OpenFile to error (parent path is a file, not a dir). Baseline 77.8%%; the marshal-error "+
			"arm is unreachable so ~88.9%% is the ceiling.", pct, floor)
	}
}

// --- C330_005 (AC-B4): the interaction suite stays green under -race ----------
//
// Regression + data-race axis. Runs the full interaction suite under -race and
// requires a clean exit. Pre-existing GREEN today; Builder must KEEP it green
// while adding the three error-branch tests. Distinct command verb (-race, no
// coverprofile) from the coverage gates for lexical diversity.
func TestC330_005_InteractionSuiteGreenRace(t *testing.T) {
	combined, code := runGoTest(t, "-race", "-count=1", interactionPkg)
	if code != 0 {
		t.Errorf("RED: internal/interaction suite fails under -race (exit=%d) — the new error-branch "+
			"tests must not break the ledger/neutralize contract:\n%s", code, tail(combined, 30))
	}
}

// ============= phasecoherence-check-nil-guard-coverage predicates ============

// --- C330_006 (AC-C1): phasecoherence.Check coverage >= 88% -------------------
//
// Behavioral + branch anchor. Check is 81.4% today; four guard/skip branches are
// dark: the nil-AgentsFS error return (distinct from the empty-FS guard already
// tested), the directory-entry skip (entry.IsDir), the nil-frontmatter skip
// (fm == nil), and the non-[]string toolsVal skip. Reaching 88% REQUIRES tests
// that drive each — a `nil` AgentsFS (not an empty fstest.MapFS), a directory
// entry under agents/, a persona with empty/no frontmatter, and a persona whose
// `tools:` value parses to a non-slice.
func TestC330_006_CheckNilGuardBranchesCovered(t *testing.T) {
	const floor = 88.0
	out := coverFuncOutput(t, coherencePkg)
	if pct := funcCoverage(t, out, "Check"); pct < floor {
		t.Errorf("RED: phasecoherence.Check coverage %.1f%% < %.1f%% — add tests for nil AgentsFS, "+
			"directory-entry skip, nil-frontmatter skip, and non-slice toolsVal skip. Baseline 81.4%%.",
			pct, floor)
	}
}

// --- C330_007 (AC-C2): the phasecoherence suite stays green under -race -------
//
// Regression + data-race axis. Runs the full phasecoherence suite under -race
// and requires a clean exit. Pre-existing GREEN today; Builder must KEEP it
// green while adding the four nil-guard tests.
func TestC330_007_PhasecoherenceSuiteGreenRace(t *testing.T) {
	combined, code := runGoTest(t, "-race", "-count=1", coherencePkg)
	if code != 0 {
		t.Errorf("RED: internal/phasecoherence suite fails under -race (exit=%d) — the new nil-guard "+
			"tests must not break the coherence-check contract:\n%s", code, tail(combined, 30))
	}
}
