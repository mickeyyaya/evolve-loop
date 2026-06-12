//go:build acs

// Package cycle296 materializes the cycle-296 acceptance criteria for the two
// committed top_n tasks (scout-report.md — soak batch #6 reliability fixes):
//
//	T1  swarm-worktreebase-guard  — worktreeBase() returns EVOLVE_WORKTREE_BASE
//	    verbatim, including RELATIVE values; the IsAbs guard lives one call deeper
//	    in addWorktree. The inbox defect (swarm-tests-relative-worktree-base) wants
//	    the refusal in worktreeBase ITSELF. Fix: change worktreeBase to
//	    (string, error), add the IsAbs check there, and remove the duplicate from
//	    addWorktree (which now propagates the error).
//	T2  resume-inserted-phase     — RunCycleFromPhase rejects every phase that is
//	    not spine-valid via Phase.IsValid(), so a checkpoint whose resumeFromPhase
//	    is an advisor-inserted phase (e.g. "mutation-gate", registered in o.runners
//	    at runtime) cannot be resumed. Fix: also accept a startPhase present in
//	    o.runners, keeping the PhaseEnd/PhaseStart rejection.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test: they invoke `go test -v` on the white-box package tests
// that call the real (unexported) worktreeBase and drive the real Orchestrator
// resume guard, then assert on the real `--- PASS:` / `--- FAIL:` lines. The
// functions under test are unexported, so an in-package white-box test driven by
// subprocess is the only way to exercise them — a magic string in a .go file can
// neither make worktreeBase return an error nor make the resume guard dispatch an
// inserted phase, so none of these is gameable by source editing alone.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.guard  worktreeBase() itself refuses a relative base   → C296_001 (named PASS line)
//	T1.green  full internal/swarm suite stays green           → C296_002 (no FAIL line)
//	T2.accept inserted-in-runners phase accepted + negatives  → C296_003 (named PASS lines)
package cycle296

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
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
	resumeOnce sync.Once
	resumeOut  string
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

// runResumeGuard runs the TestRunCycleFromPhase white-box tests (which drive the
// real Orchestrator resume guard against a runtime-registered inserted phase),
// verbose, ONCE per predicate process.
func runResumeGuard(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	resumeOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "TestRunCycleFromPhase", "./internal/core/")
		resumeOut = stdout + "\n" + stderr
	})
	return resumeOut
}

// ===================== T1 — swarm worktreeBase guard ========================

// --- C296_001 (T1.guard): worktreeBase() ITSELF refuses a relative base -------
//
// Behavioral: gates on a real `--- PASS: TestWorktreeBase_RelativeEnvReturnsError`
// line. That white-box test calls the unexported worktreeBase directly with a
// relative EVOLVE_WORKTREE_BASE and asserts it returns ("", error mentioning
// "absolute"). A magic string cannot produce a named PASS line for a function
// that still returns the relative path verbatim.
//
// RED baseline: worktreeBase returns a single string (no error), so the white-box
// test does not even compile → no PASS line for it. GREEN requires the
// (string, error) signature with the IsAbs guard inside worktreeBase.
func TestC296_001_WorktreeBaseRefusesRelativeBase(t *testing.T) {
	out := runSwarmWorktreeBase(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a TestWorktreeBase* test FAILs:\n%s", tail(out, 40))
	}
	if !topLevelPassed(out, "TestWorktreeBase_RelativeEnvReturnsError") {
		t.Errorf("RED: TestWorktreeBase_RelativeEnvReturnsError did not PASS — worktreeBase " +
			"does not yet refuse a relative EVOLVE_WORKTREE_BASE itself (guard still only in addWorktree)")
	}
	// Auxiliary anti-duplication check (not RED-discriminating), FUNCTION-SCOPED
	// so it cannot rot when later cycles legitimately add IsAbs elsewhere in the
	// file (the original file-wide ==2 count rotted within a day — 0c210b52):
	// the IsAbs guard belongs in worktreeBase and must NOT be duplicated in
	// addWorktree.
	prov := filepath.Join(goDir(t), "internal", "swarm", "provision.go")
	if n, err := acsassert.CountInGoFunc(prov, "addWorktree", "filepath.IsAbs"); err != nil || n != 0 {
		t.Errorf("addWorktree must contain NO filepath.IsAbs (guard lives in worktreeBase, "+
			"not duplicated): found %d, err=%v", n, err)
	}
	if n, err := acsassert.CountInGoFunc(prov, "worktreeBase", "filepath.IsAbs"); err != nil || n < 1 {
		t.Errorf("worktreeBase must carry the filepath.IsAbs guard itself: found %d, err=%v", n, err)
	}
}

// --- C296_002 (T1.green): the whole internal/swarm suite stays green ----------
//
// Anti-no-op regression gate: the signature change touches addWorktree and the
// existing TestWorktreeBase_EnvOverride / _DefaultPath callers. Running the full
// swarm suite (real provisioner, isolated temp repos) and asserting no `--- FAIL:`
// line ensures the refactor did not break the env-override / default / idempotent
// / cleanup behaviors. A FAIL line is a real test failure no source string fakes.
func TestC296_002_SwarmSuiteGreen(t *testing.T) {
	dir := goDir(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "-v", "./internal/swarm/")
	out := stdout + "\n" + stderr
	if anyFailRe.MatchString(out) || code != 0 {
		t.Errorf("RED/REGRESSION: internal/swarm suite is not green (exit=%d):\n%s", code, tail(out, 50))
	}
}

// ===================== T2 — resume inserted-phase acceptance ==================

// --- C296_003 (T2.accept): RunCycleFromPhase accepts an inserted-in-runners phase
//
// Behavioral: gates on real `--- PASS:` lines for both the positive and the
// negative axis of the resume-guard fix:
//   - TestRunCycleFromPhase_InsertedPhaseInRunnersAccepted: a phase present in
//     o.runners but not spine-valid is DISPATCHED (runner.Run called) rather than
//     rejected as "invalid resume phase".
//   - TestRunCycleFromPhase_PhaseStartRejected: PhaseStart is still rejected
//     (negative parity with the existing PhaseEnd guard).
//
// Both are driven by the real Orchestrator. RED baseline: the guard is a hard
// `!startPhase.IsValid()`, so the inserted phase is rejected before dispatch and
// the positive test FAILs (no PASS line). GREEN requires the o.runners acceptance
// branch while preserving the PhaseEnd/PhaseStart rejection.
func TestC296_003_ResumeAcceptsInsertedPhase(t *testing.T) {
	out := runResumeGuard(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a TestRunCycleFromPhase* test FAILs:\n%s", tail(out, 40))
	}
	if !topLevelPassed(out, "TestRunCycleFromPhase_InsertedPhaseInRunnersAccepted") {
		t.Errorf("RED: TestRunCycleFromPhase_InsertedPhaseInRunnersAccepted did not PASS — " +
			"the resume guard still rejects an advisor-inserted phase registered in o.runners")
	}
	if !topLevelPassed(out, "TestRunCycleFromPhase_PhaseStartRejected") {
		t.Errorf("RED/REGRESSION: TestRunCycleFromPhase_PhaseStartRejected did not PASS — " +
			"PhaseStart must remain a rejected resume target after the guard change")
	}
}
