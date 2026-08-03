//go:build acs

// Package cycle1233 materialises the acceptance criteria for this lane's single
// triage-COMMITTED (## top_n) task:
//
//	artifact-ready-crosspoll-debounce
//
// artifactDetector.poll completes on the FIRST non-empty read of the
// deliverable (completion.go:157). Cycle-1198's gate rejected a scout-report.md
// caught mid Write→Edit — sections present, trailing verdict sentinel not yet
// appended — that parsed perfectly moments later. The deliverable-side grace
// window already shipped but covers absence/emptiness only; a "parses fine,
// wrong content" read is not retried by design. The fix closes it at the SOURCE:
// identical (size, mtime) across artifactStableTicks CONSECUTIVE poll ticks
// (~2s apart) before ready — the artifact twin of stdoutDetector's
// stdoutIdlePolls.
//
// Predicate strategy. artifactDetector, its poll method, and the wait loop are
// all UNEXPORTED, so these predicates cannot import them; each one instead
// shells `go test -run -v` at the single package internal/bridge over the RED
// contract authored this cycle in completion_debounce_test.go, and asserts the
// per-test `--- PASS:` receipt. The receipt check is load-bearing anti-gaming:
// `go test -run <deleted-test>` matches nothing and exits 0, so an exit-code-only
// predicate would go GREEN if Builder deleted the contract instead of satisfying
// it (cycle-1113 lesson).
//
//	001 — the debounce itself: positive settle + BOTH negative axes (a growing
//	      file, and the same-size/different-mtime rewrite that a size-only key is
//	      blind to). The anti-no-op half: first-sight completion fails here.
//	002 — the ctx-cancel short-circuit, with its own negative (cancellation must
//	      not manufacture completion from an absent artifact). Separable sub-fix:
//	      without it the debounce launders finished sessions into ExitArtifactTimeout.
//	003 — the single-shot relocation diagnostic survives the unstable tick that
//	      observed it, and relocation is not an exemption from the window.
//	004 — CALLER PROOF + fixture budget: the debounce is reached from the real
//	      production wait loop (Engine.LaunchArgs → runTmuxREPL → detector.poll,
//	      driver_tmux_repl.go:601), and the short-ArtifactTimeoutS fixture the
//	      prior console attempt was rolled back over stays green (MUST-ALSO (c)).
//
// Diversity: 001 carries one positive and two independent negatives, 002 and 003
// each pair a positive with a negative, 004 is end-to-end behavioural through the
// production entry point rather than the unit seam.
package cycle1233

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// bridgePkg is the ONE named package these predicates run. Never a `./...`
// sweep: a whole-repo run is the regression suite's job and is a false-red
// generator under fleet load.
const bridgePkg = "./internal/bridge/"

// goDir is the worktree's go module root — predicates read the CYCLE's source,
// not main's (worktree isolation; acsassert.RepoRoot resolves the worktree).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runContract runs `go test -C <worktree>/go -v -run ^(names...)$ ./internal/bridge/`
// and reports whether EVERY named test both ran and passed.
//
// Two failure shapes are distinguished deliberately:
//   - code < 0 is a genuine "could not launch" and is fatal (cycle-574 lesson);
//     a compile failure in the target package — the expected RED signal before
//     Builder implements — is a NON-ZERO EXIT, not a launch failure.
//   - a zero exit with a missing `--- PASS: <name>` receipt means the test is
//     gone, not that it passed. That is reported as a miss, not a pass.
func runContract(t *testing.T, names ...string) (ok bool, missing []string, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-v",
		"-run", "^("+strings.Join(names, "|")+")$", bridgePkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s: code=%d err=%v\n%s", bridgePkg, code, err, tail(out, 30))
	}
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			missing = append(missing, n)
		}
	}
	return code == 0 && len(missing) == 0, missing, out
}

// tail returns the last n lines — diagnostics stay readable in the verdict.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// report renders a uniform RED message naming both halves of a failure: the
// contract that did not hold, and any test that vanished rather than passing.
func report(t *testing.T, what string, missing []string, out string) {
	t.Helper()
	if len(missing) > 0 {
		t.Errorf("RED: %s — and these contract tests did not report PASS (deleted or skipped): %v\n%s",
			what, missing, tail(out, 40))
		return
	}
	t.Errorf("RED: %s\n%s", what, tail(out, 40))
}

// TestC1233_001_ArtifactReadyRequiresCrossPollStability pins AC-1. Positive: a
// settled deliverable completes, but never on the tick it is first seen.
// Negative (the anti-no-op axis, and what makes this predicate un-passable by a
// no-op): a file still growing, and a same-SIZE rewrite with a fresh mtime, must
// each stay incomplete for as long as they keep changing. The mtime case is the
// one a size-only key silently degrades on.
func TestC1233_001_ArtifactReadyRequiresCrossPollStability(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
		"TestArtifactStableTicks_IsAMeaningfulWindow",
	)
	if !ok {
		report(t, "artifactDetector.poll still completes on the first non-empty read (or its "+
			"stability key is size-only) — the cycle-1198 mid-Write→Edit truncated read is still accepted",
			missing, out)
	}
}

// TestC1233_002_CtxCancelShortCircuitsTheWindow pins AC-2. The wait loop's one
// post-cancellation poll exists so a finished session is not laundered into
// ExitArtifactTimeout; a debounce that demands a fresh window on that last look
// converts every teardown-at-the-finish-line into a false timeout. The paired
// negative keeps the short-circuit from becoming an unconditional "cancelled ⇒
// ready".
func TestC1233_002_CtxCancelShortCircuitsTheWindow(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
	)
	if !ok {
		report(t, "a cancelled context does not short-circuit the stability window (or it "+
			"short-circuits into a false completion with no artifact on disk)", missing, out)
	}
}

// TestC1233_003_RelocationDiagnosticSurvivesUnstableTick pins AC-3. artifactReady
// returns relocatedFrom exactly ONCE — on the tick it moves a non-canonical write
// into place. If the debounce drops the note of a not-yet-stable tick, the
// "agent wrote to the wrong place" signal is lost forever.
func TestC1233_003_RelocationDiagnosticSurvivesUnstableTick(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_RelocationNoteSurvivesUntilStable",
	)
	if !ok {
		report(t, "the single-shot relocation diagnostic is swallowed by the unstable tick that "+
			"observed it (or relocation bypasses the stability window entirely)", missing, out)
	}
}

// TestC1233_004_DebounceReachedFromWaitLoopAndFixturesHold is the CALLER PROOF
// plus the fixture-budget guard. A debounce implemented on a struct the driver
// never consults is dead code, so the first test drives the real production
// entry point (Engine.LaunchArgs → runTmuxREPL → detector.poll) and asserts a
// continuously-rewritten deliverable does NOT complete the phase. The second is
// the short-ArtifactTimeoutS fixture whose underrun is the documented reason the
// prior console attempt at this fix was rolled back (inbox MUST-ALSO (c)) — an
// extra required tick must not strand it.
func TestC1233_004_DebounceReachedFromWaitLoopAndFixturesHold(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
		"TestRunTmuxREPL_ExtendNoEscalationReport",
	)
	if !ok {
		report(t, "the stability window is not reached from the production wait loop, or the "+
			"extra tick underruns the ArtifactTimeoutS=2 fixture (the rollback cause)", missing, out)
	}
}
