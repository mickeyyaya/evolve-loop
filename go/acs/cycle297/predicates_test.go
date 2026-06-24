//go:build acs

// Package cycle297 materializes the cycle-297 acceptance criteria for the two
// committed top_n tasks (scout-report.md — soak batch #6 reliability fixes):
//
//	T1  worktreebase-relative-projectroot-guard — worktreeBase() guards
//	    EVOLVE_WORKTREE_BASE (env path) for absoluteness but NOT the default path.
//	    When the env var is unset and projectRoot is relative (e.g. "."),
//	    filepath.Join(".", ".evolve", "worktrees") = ".evolve/worktrees" is
//	    returned with a nil error. This is the last gap of the inbox defect
//	    swarm-tests-relative-worktree-base (cycle 296 only moved the env-var
//	    check). Fix: guard filepath.IsAbs(projectRoot) in the default branch too.
//	T2  cli-version-freeze-claude — defaultSelfUpdateEvidence switches on
//	    bin=="codex" only, so a host where claude is NOT brew-pinned silently
//	    passes the version-freeze readiness check. claude 2.1.173 self-updated
//	    mid-soak (removed `esc to interrupt`), breaking PaneBusy detection →
//	    exit=81 in cycles 286/288/289/291 (inbox HIGH claude-cli-version-freeze).
//	    Fix: add a claude clause checking ~/.claude/settings.json (analogous to
//	    codex's ~/.codex/version.json).
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test: they invoke `go test -v` on the white-box package tests
// that call the real (unexported) worktreeBase and drive the real freeze
// Specification through Run, then assert on the real `--- PASS:` / `--- FAIL:`
// lines. The functions under test are unexported, so an in-package white-box test
// driven by subprocess is the only way to exercise them — a magic string in a .go
// file can neither make worktreeBase return an error on a relative default path
// nor make the freeze check HALT for claude, so none of these is gameable by
// source editing alone.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1  worktreeBase(".") returns ("", err mentioning "absolute")  → C297_001
//	    + full internal/swarm suite stays green
//	T2  real defaultSelfUpdateEvidence("claude") + end-to-end HALT  → C297_002
//	    + full internal/looppreflight suite stays green
package cycle297

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

// topLevelPassed reports whether a `--- PASS: <name>` line names exactly `name`.
func topLevelPassed(out, name string) bool {
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// --- shared one-shot subprocess runners (one `go test` per scope, reused) ---

var (
	swarmOnce  sync.Once
	swarmOut   string
	freezeOnce sync.Once
	freezeOut  string
)

// runSwarmWorktreeBase runs the TestWorktreeBase* white-box tests (which call the
// real unexported worktreeBase), verbose, ONCE per predicate process. Scoped via
// -run so an unrelated swarm change cannot false-RED this gate.
func runSwarmWorktreeBase(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	swarmOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "TestWorktreeBase", "./internal/swarm/")
		swarmOut = stdout + "\n" + stderr
	})
	return swarmOut
}

// runFreezeClaude runs the claude version-freeze white-box tests (the real
// defaultSelfUpdateEvidence unit tests + the end-to-end Run HALT test, all
// HOME-redirected so they are host-independent), verbose, ONCE per process.
func runFreezeClaude(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	freezeOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "VersionFreeze_Claude|DefaultSelfUpdateEvidence_Claude",
			"./internal/looppreflight/")
		freezeOut = stdout + "\n" + stderr
	})
	return freezeOut
}

// ===================== T1 — worktreeBase default-path guard ==================

// --- C297_001 (T1): worktreeBase() refuses a relative DEFAULT-path projectRoot
//
// Behavioral: gates on a real `--- PASS: TestWorktreeBase_RelativeProjectRootRefused`
// line. That white-box test clears EVOLVE_WORKTREE_BASE (forcing the default
// branch) and calls the unexported worktreeBase(".") directly, asserting it
// returns ("", error mentioning "absolute"). A magic string cannot produce a
// named PASS line for a function that still returns ".evolve/worktrees" verbatim.
//
// RED baseline: worktreeBase(".") returns (".evolve/worktrees", nil) — the
// white-box test FAILs (no PASS line). GREEN requires the IsAbs(projectRoot)
// guard in the default branch. The no-FAIL check is the anti-regression axis:
// the existing TestWorktreeBase_AbsoluteOverride/_DefaultPath/_RelativeOverrideReturnsError
// callers must stay green through the change.
func TestC297_001_WorktreeBaseRefusesRelativeDefaultRoot(t *testing.T) {
	out := runSwarmWorktreeBase(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a TestWorktreeBase* test FAILs:\n%s", tail(out, 40))
	}
	if !topLevelPassed(out, "TestWorktreeBase_RelativeProjectRootRefused") {
		t.Errorf("RED: TestWorktreeBase_RelativeProjectRootRefused did not PASS — worktreeBase " +
			"does not yet refuse a relative projectRoot on the default (no-env) path")
	}
}

// --- C297_001b (T1.green): the whole internal/swarm suite stays green ---------
//
// Anti-no-op regression gate: the default-branch guard sits on the hot path of
// every provisioner call. Running the full swarm suite (real provisioner,
// isolated temp repos) and asserting no `--- FAIL:` line ensures the new guard
// did not break the env-override / default / idempotent / cleanup behaviors. A
// FAIL line is a real test failure no source string fakes.
func TestC297_001b_SwarmSuiteGreen(t *testing.T) {
	dir := goDir(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "-v", "./internal/swarm/")
	out := stdout + "\n" + stderr
	if anyFailRe.MatchString(out) || code != 0 {
		t.Errorf("RED/REGRESSION: internal/swarm suite is not green (exit=%d):\n%s", code, tail(out, 50))
	}
}

// ===================== T2 — claude cli-version-freeze ========================

// --- C297_002 (T2): the freeze registry recognizes claude as self-updating ----
//
// Behavioral: gates on real `--- PASS:` lines for both the unit and the
// end-to-end axis of the claude registry addition:
//   - TestDefaultSelfUpdateEvidence_ClaudePresent: the REAL defaultSelfUpdateEvidence
//     (HOME redirected to a temp dir holding ~/.claude/settings.json) reports
//     (true, evidence-path, nil) for "claude".
//   - TestRun_VersionFreeze_ClaudeUnpinnedRealEvidence_Halts: a claude-tmux
//     profile wired to the REAL evidence function with an unpinned claude HALTs
//     with `brew pin claude` guidance.
//
// Both drive the real code, not an injected stub. RED baseline:
// defaultSelfUpdateEvidence ignores every binary != "codex", so claude reports
// no evidence → the unit test FAILs and the end-to-end check returns Pass
// instead of Halt (no PASS line for either). GREEN requires the claude clause
// checking ~/.claude/settings.json. The no-FAIL check guards the negative-axis
// companions (ClaudeAbsent / ClaudeNoSettings) so the fix does not over-fire.
func TestC297_002_FreezeRecognizesClaude(t *testing.T) {
	out := runFreezeClaude(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a claude version-freeze test FAILs:\n%s", tail(out, 40))
	}
	if !topLevelPassed(out, "TestDefaultSelfUpdateEvidence_ClaudePresent") {
		t.Errorf("RED: TestDefaultSelfUpdateEvidence_ClaudePresent did not PASS — " +
			"defaultSelfUpdateEvidence does not yet recognize claude's ~/.claude/settings.json")
	}
	if !topLevelPassed(out, "TestRun_VersionFreeze_ClaudeUnpinnedRealEvidence_Halts") {
		t.Errorf("RED: TestRun_VersionFreeze_ClaudeUnpinnedRealEvidence_Halts did not PASS — " +
			"an unpinned self-updating claude-tmux does not yet HALT the batch")
	}
}

// --- C297_002b (T2.green): the whole internal/looppreflight suite stays green -
//
// Anti-no-op regression gate: the claude clause sits in defaultSelfUpdateEvidence,
// shared by every freeze check. Running the full looppreflight suite and
// asserting no `--- FAIL:` line ensures the codex evidence/halt/pass behaviors
// and the unregistered-binary default are unaffected (no codex regressions).
func TestC297_002b_LoopPreflightSuiteGreen(t *testing.T) {
	dir := goDir(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "-v", "./internal/looppreflight/")
	out := stdout + "\n" + stderr
	if anyFailRe.MatchString(out) || code != 0 {
		t.Errorf("RED/REGRESSION: internal/looppreflight suite is not green (exit=%d):\n%s", code, tail(out, 50))
	}
}
