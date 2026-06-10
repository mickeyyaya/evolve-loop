//go:build acs

// Package cycle276 materializes the cycle-276 acceptance criteria for the three
// committed top_n tasks (scout-report.md — "bridge abstraction hardening"):
//
//	T1  bridge-tmux-controller-fixture-tests — FakeTmuxController + a multi-frame
//	                                            fixture corpus unit-tests the 885-line
//	                                            driver_tmux_repl state machine; tmux.go
//	                                            stops being an all-0% drag-anchor.
//	T2  bridge-codex-boot-sync-gate          — codex's pre-REPL update-menu nag no
//	                                            longer swallows the injected prompt;
//	                                            shell-spill panes are classified fatal.
//	T3  bridge-profile-contract-symmetry     — the runner fast-fails with a path-named
//	                                            diagnostic on a missing profile instead
//	                                            of letting the bridge exit a terse 10.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test as a real subprocess — `go test -v` over the bridge /
// recovery / runner packages and a `go tool cover` total — and assert on the real
// `--- PASS: <name>` lines, sub-case counts, and the coverage numbers the
// builder's new in-package tests produce. A magic string in a .go file can neither
// produce a named PASS line nor move a coverage number, so none of these is
// gameable by source editing alone (the established cycle-274 pattern).
//
// Convention (cycle-274): the BUILDER authors the in-package unit tests named
// below (TestTmuxFixture*, TestCodexUpdateMenuDismiss, TestFatalPaneShellSpill,
// TestRunnerMissingProfileFastFail, …) alongside the production code; these ACS
// predicates GATE on those tests running + passing with the required adversarial
// diversity. RED at baseline: none of those tests exist yet → no PASS lines →
// every predicate fails for the right reason. The coverage predicate (C276_003)
// is RED against the measured baseline (tmux.go = 1/9 funcs covered: stripANSI).
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.a FakeTmuxController defined            → C276_001 (fixture cases can't pass without it)
//	T1.b >= 3 fixture sub-cases PASS           → C276_001
//	T1.c tmux.go no longer 0% across the board → C276_003 (controller funcs now covered)
//	T1.d boot-timeout negative case covered    → C276_002 (a timeout fixture case present)
//	T2.a TestCodexUpdateMenuDismiss PASS       → C276_004 (+ no-FAIL rider = T2.d)
//	T2.b TestFatalPaneShellSpill PASS          → C276_005
//	T2.c TestFatalPaneNoFalsePositive PASS     → C276_006
//	T2.d full bridge suite PASS                → C276_004 (anyFail rider over the bridge suite)
//	T3.a TestRunnerMissingProfileFastFail PASS → C276_008 (+ no-FAIL rider = T3.c)
//	T3.b TestRunnerMissingProfileDiagnostic PASS (output names the missing path) → C276_009
//	T3.c full runner suite PASS                → C276_008 (anyFail rider over the runner suite)
package cycle276

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// --- shared one-shot subprocess runners (one `go test` per package, reused) ---

var (
	bridgeOnce   sync.Once
	bridgeOut    string
	recoveryOnce sync.Once
	recoveryOut  string
	runnerOnce   sync.Once
	runnerOut    string
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runBridge runs the bridge package suite (verbose, cache-defeated) ONCE per
// predicate process. The builder's FakeTmuxController fixture tests + the
// codex boot-sync gate test land here.
func runBridge(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	bridgeOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v", "./internal/bridge/")
		bridgeOut = stdout + "\n" + stderr
	})
	return bridgeOut
}

// runRecovery runs the recovery package suite ONCE. The new shell-spill fatal
// signatures (recovery.SeedDetector) and their no-false-positive companion may
// land here instead of (or alongside) the bridge-level fatalPaneVerdict test.
func runRecovery(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	recoveryOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v", "./internal/recovery/")
		recoveryOut = stdout + "\n" + stderr
	})
	return recoveryOut
}

// runRunner runs the phases/runner package suite ONCE. The runner fast-fail +
// diagnostic tests land here; the full suite green-ness is the T3.c regression
// floor.
func runRunner(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	runnerOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v", "./internal/phases/runner/")
		runnerOut = stdout + "\n" + stderr
	})
	return runnerOut
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

// passNames returns every passing test path (top-level and sub) in `out`.
func passNames(out string) []string {
	var names []string
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		names = append(names, m[1])
	}
	return names
}

// topLevelPassed reports whether a column-0 `--- PASS: <name>` line is present
// (a sub-test PASS line is indented, so the no-leading-slash check rejects it).
func topLevelPassed(out, name string) bool {
	for _, n := range passNames(out) {
		if n == name {
			return true
		}
	}
	return false
}

// subPasses returns the distinct passing subtest names under `parent` (the
// `<parent>/<sub>` PASS paths). Two distinct passing sub-cases prove a table
// exercised more than one row — the lever that forces the adversarial negative
// alongside the positive (skills/adversarial-testing §6); a positive-only test
// is gameable by a no-op.
func subPasses(out, parent string) map[string]bool {
	seen := map[string]bool{}
	prefix := parent + "/"
	for _, n := range passNames(out) {
		if strings.HasPrefix(n, prefix) {
			seen[strings.TrimPrefix(n, prefix)] = true
		}
	}
	return seen
}

// fixtureCases returns the distinct passing test CASES whose path starts with
// `prefix`. A "case" is a sub-test (`Parent/sub`) OR a leaf top-level test with
// no sub-tests of its own — a parent that fans out into sub-tests is NOT itself
// a case (only its sub-rows are). This counts the same way whether the builder
// writes one table-driven TestTmuxFixture with N rows or N separate
// TestTmuxFixture* functions — both satisfy the scout's "-run TestTmuxFixture →
// >= 3 sub-cases".
func fixtureCases(out, prefix string) []string {
	names := passNames(out)
	parents := map[string]bool{}
	for _, n := range names {
		if i := strings.Index(n, "/"); i >= 0 {
			parents[n[:i]] = true
		}
	}
	seen := map[string]bool{}
	for _, n := range names {
		if !strings.HasPrefix(n, prefix) {
			continue
		}
		if strings.Contains(n, "/") { // a sub-case
			seen[n] = true
			continue
		}
		if !parents[n] { // a leaf top-level test (no sub-rows)
			seen[n] = true
		}
	}
	out2 := make([]string, 0, len(seen))
	for n := range seen {
		out2 = append(out2, n)
	}
	return out2
}

// ===================== T1 — FakeTmuxController + fixture corpus ===============

// --- C276_001 (T1.a, T1.b): the FakeTmuxController-driven fixture corpus runs
// the driver_tmux_repl state machine through >= 3 distinct cases ---
//
// Behavioral: the builder's `TestTmuxFixture*` tests drive runTmuxREPL with the
// new scriptable FakeTmuxController (per-method frame queue, panic-on-underrun)
// over multi-frame fixtures derived from the two cycle-274 wedge scrollbacks.
// Requiring >= 3 distinct passing cases is the scout's exact verifiableBy and
// proves the fake exists and exercises more than the happy path (a single
// positive case is gameable). The no-FAIL rider guards against the new tests
// regressing the existing bridge suite. RED: no TestTmuxFixture* PASS lines.
func TestC276_001_TmuxFixtureCorpusDrivesStateMachine(t *testing.T) {
	out := runBridge(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: bridge suite has a FAIL line:\n%s", tail(out, 60))
	}
	cases := fixtureCases(out, "TestTmuxFixture")
	if len(cases) < 3 {
		t.Errorf("RED: TestTmuxFixture* has %d passing case(s) %v, want >= 3 "+
			"(the FakeTmuxController-driven boot-success / boot-timeout / artifact-delivery corpus — scout T1 verifiableBy)",
			len(cases), cases)
	}
}

// --- C276_002 (T1.d): the fixture corpus covers the boot-timeout NEGATIVE ---
//
// The strongest anti-no-op signal (adversarial-testing §6): a fixture in which
// the prompt marker NEVER appears must drive runTmuxREPL to ExitREPLBootTimeout.
// A corpus that only proves the happy path does not require the boot loop's
// deadline branch. Behavioral: at least one passing TestTmuxFixture case names a
// timeout scenario. RED: no such case.
func TestC276_002_TmuxFixtureBootTimeoutNegative(t *testing.T) {
	out := runBridge(t)
	timeoutRe := regexp.MustCompile(`(?i)timeout`)
	for _, c := range fixtureCases(out, "TestTmuxFixture") {
		if timeoutRe.MatchString(c) {
			return // a boot-timeout fixture case ran + passed
		}
	}
	t.Errorf("RED: no passing TestTmuxFixture case matches /timeout/ — the boot-timeout " +
		"negative (marker never appears → ExitREPLBootTimeout) is uncovered (T1.d)")
}

// --- C276_003 (T1.c): tmux.go is no longer an all-0% drag-anchor ---
//
// Load-bearing/objective: the number is produced by REALLY running the bridge
// suite over the package with -coverprofile, so it can only move once the
// builder's FakeTmuxController methods (and/or real execTmux exercises) are
// actually covered by the new fixture tests. Un-gameable by source editing.
// Baseline (measured 2026-06-10): exactly 1 of 9 tmux.go funcs has >0% coverage
// (stripANSI=100%; the 8 execTmux methods are 0.0%). RED: covered count == 1.
func TestC276_003_TmuxGoControllerCovered(t *testing.T) {
	dir := goDir(t)
	prof := filepath.Join(t.TempDir(), "cover.out")
	_, tErr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-short", "-count=1", "-coverprofile="+prof, "./internal/bridge/")
	funcOut, cErr, _, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+prof)
	covered, total := 0, 0
	for _, ln := range strings.Split(funcOut, "\n") {
		if !strings.Contains(ln, "/tmux.go:") {
			continue
		}
		total++
		fields := strings.Fields(ln)
		if len(fields) == 0 {
			continue
		}
		if fields[len(fields)-1] != "0.0%" {
			covered++
		}
	}
	if total == 0 {
		t.Fatalf("RED: no `/tmux.go:` rows in `go tool cover -func` output — profile not produced.\ntest stderr:\n%s\ncover stderr:\n%s",
			tail(tErr, 30), tail(cErr, 30))
	}
	if covered < 3 {
		t.Errorf("RED: tmux.go has %d/%d funcs with >0%% coverage, want >= 3 "+
			"(baseline 1 = stripANSI only; the FakeTmuxController / execTmux controller methods must be exercised by the fixture corpus — T1.c)",
			covered, total)
	}
}

// ===================== T2 — codex boot-sync injection gate ====================

// --- C276_004 (T2.a, T2.d): the boot-sync gate dismisses codex's update-menu
// BEFORE delivering the task prompt ---
//
// THE batch-fatal defect (inbox: codex-update-menu-swallows-injection). codex
// 0.138 renders an interactive update-menu before its REPL prompt; the current
// `strings.Contains(pane, promptMarker)` boot check fires on the menu and the
// driver pastes the prompt into it, wedging the session. Behavioral: the
// builder's `TestCodexUpdateMenuDismiss` drives runTmuxREPL with a
// FakeTmuxController returning a menu frame THEN the real prompt, and asserts the
// gate dismisses the menu (a Skip keypress) and only injects at the real prompt.
// Requiring >= 2 sub-cases forces BOTH the menu-present (dismiss) path AND the
// menu-absent (no spurious Skip) negative. The anyFail rider over the bridge
// suite is the T2.d "full bridge suite PASS" floor. RED: test absent.
func TestC276_004_CodexUpdateMenuDismissed(t *testing.T) {
	out := runBridge(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: bridge suite has a FAIL line (T2.d full-suite floor):\n%s", tail(out, 60))
	}
	if !topLevelPassed(out, "TestCodexUpdateMenuDismiss") {
		t.Errorf("RED: TestCodexUpdateMenuDismiss did not run+PASS — the codex boot-sync gate " +
			"(dismiss update-menu before injecting the prompt) is not implemented/tested (T2.a)")
	}
	if subs := subPasses(out, "TestCodexUpdateMenuDismiss"); len(subs) < 2 {
		t.Errorf("RED: TestCodexUpdateMenuDismiss has %d passing sub-case(s), want >= 2 "+
			"(menu-present → dismissed before inject, AND menu-absent → no spurious Skip keypress)", len(subs))
	}
}

// --- C276_005 (T2.b): shell-spill panes are classified fatal ---
//
// cycle-274 observed codex self-update / brew-upgrade leaving the pane in a bare
// shell with `zsh: command not found` and zsh `quote>` / `bquote>` continuation
// prompts — but only `: command not found` was a seeded fatal signature; the
// continuation prompts slipped through, so the stop-reviewer burned the full
// maxExtends backstop on a dead shell. Behavioral: the builder's
// `TestFatalPaneShellSpill` proves the (extended) registry now classifies these
// as fatal — exercised either via recovery.Detect or the bridge fatalPaneVerdict
// integration. Requiring >= 2 sub-cases forces more than one spill signature
// (e.g. the quote>/bquote> continuation AND the command-not-found form). RED:
// test absent in both the bridge and recovery suites.
func TestC276_005_FatalPaneShellSpillClassified(t *testing.T) {
	out := runBridge(t) + "\n" + runRecovery(t)
	if !topLevelPassed(out, "TestFatalPaneShellSpill") {
		t.Errorf("RED: TestFatalPaneShellSpill did not run+PASS — shell-spill panes " +
			"(quote>/bquote> continuation, command-not-found) are not classified fatal (T2.b)")
	}
	if subs := subPasses(out, "TestFatalPaneShellSpill"); len(subs) < 2 {
		t.Errorf("RED: TestFatalPaneShellSpill has %d passing sub-case(s), want >= 2 "+
			"(distinct spill signatures — e.g. a zsh quote>/bquote> continuation AND a command-not-found line)", len(subs))
	}
}

// --- C276_006 (T2.c): the new fatal signatures do NOT false-positive a healthy
// pane ---
//
// The adversarial NEGATIVE for C276_005: a working agent's pane can legitimately
// contain the word "quote" or discuss a missing command without being a dead
// shell. The new signatures must be specific enough (anchored substrings: the
// shell's OWN `quote>`/`bquote>` continuation prompt and the colon-prefixed
// `: command not found`) that a healthy pane is classified Unknown, not fatal.
// Without this pin a signature that simply matched "quote" would pass C276_005
// while killing every healthy agent that mentions a quote. Behavioral: the
// builder's `TestFatalPaneNoFalsePositive` must run+PASS. RED: test absent.
func TestC276_006_FatalPaneNoFalsePositive(t *testing.T) {
	out := runBridge(t) + "\n" + runRecovery(t)
	if !topLevelPassed(out, "TestFatalPaneNoFalsePositive") {
		t.Errorf("RED: TestFatalPaneNoFalsePositive did not run+PASS — the new shell-spill " +
			"signatures must NOT classify a healthy pane that merely mentions 'quote'/'command' as fatal (T2.c)")
	}
}

// ===================== T3 — runner profile contract symmetry ==================

// --- C276_008 (T3.a, T3.c): the runner fast-fails on a missing profile ---
//
// Inbox defect (dispatchable-agent-profile-completeness): the runner silently
// tolerates loader.Get returning an error (prof=nil) and then hands the
// nonexistent profilePath to the bridge, which exits a terse ExitBadFlags=10 with
// no diagnostic pointing at the missing file. Behavioral: the builder's
// `TestRunnerMissingProfileFastFail` proves the runner now detects the missing
// profile and fast-fails BEFORE dispatch. The anyFail rider over the full runner
// suite is the T3.c regression floor — it pins that legitimately profile-less
// phases still dispatch (existing runner tests must stay green). RED: test absent.
func TestC276_008_RunnerMissingProfileFastFail(t *testing.T) {
	out := runRunner(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: runner suite has a FAIL line (T3.c full-suite floor — "+
			"the fast-fail must not break legitimately profile-less phases):\n%s", tail(out, 60))
	}
	if !topLevelPassed(out, "TestRunnerMissingProfileFastFail") {
		t.Errorf("RED: TestRunnerMissingProfileFastFail did not run+PASS — the runner still " +
			"tolerates a missing profile and defers the failure to the bridge's terse exit 10 (T3.a)")
	}
}

// --- C276_009 (T3.b): the fast-fail diagnostic NAMES the missing profile path ---
//
// The asymmetry the defect is about is not just "fail earlier" but "fail
// legibly": the operator must see WHICH profile path was missing, not an
// opaque ExitBadFlags=10. Behavioral: the builder's
// `TestRunnerMissingProfileDiagnostic` must run+PASS, asserting the runner's
// error output contains the missing profile path. RED: test absent.
func TestC276_009_RunnerMissingProfileDiagnostic(t *testing.T) {
	out := runRunner(t)
	if !topLevelPassed(out, "TestRunnerMissingProfileDiagnostic") {
		t.Errorf("RED: TestRunnerMissingProfileDiagnostic did not run+PASS — the fast-fail " +
			"diagnostic must name the missing profile path (not a terse exit code) (T3.b)")
	}
}

// --- small helpers ---

// tail returns the last n lines of s (keeps RED failure output bounded).
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
