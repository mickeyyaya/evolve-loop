//go:build acs

// Package cycle1236 materialises the acceptance criteria for this lane's two
// triage-COMMITTED (## top_n) tasks:
//
//	completion-contract-cancel-parity   — the cycle's actual work
//	artifact-ready-crosspoll-debounce   — ALREADY LANDED at HEAD 841e676f; these
//	                                      predicates verify-and-close it rather
//	                                      than re-opening it (see 004)
//
// The parity defect. driver_tmux_repl.go:576 takes ONE final completion poll
// after its context is cancelled, so a session that finished at the buzzer is
// not laundered into ExitArtifactTimeout — and it hands that poll the
// ALREADY-CANCELLED ctx, reasoning (comment at :566-575) that "the artifact
// detector is a pure file stat, so the dead ctx cannot fail this last look".
// The same line dispatches polymorphically to three completionDetector
// implementations (built at :481 from cfg.Completion) and the claim holds for
// exactly one of them. stdoutDetector shells CapturePane(ctx, …) and
// gitEvidenceDetector shells Runner(ctx, "git", …); exec.CommandContext refuses
// to fork on a dead context, both detectors correctly swallow the transport
// error as "not ready", and a DELIVERED stdout/git phase exits 81.
//
// The coupling trap the predicates also guard. artifactDetector's short-circuit
// (completion.go:213) keys on ctx.Err() != nil. Simply passing a live context
// to the final poll switches that short-circuit OFF, and the detector then
// demands a 2-tick stability window it can never accrue in a single call — the
// naive fix regresses the one contract that already works. 003 is the guard.
//
// Predicate strategy. artifactDetector, stdoutDetector, gitEvidenceDetector,
// their poll methods and the wait loop are all UNEXPORTED, so these predicates
// cannot import them. Each instead shells `go test -run -v` at the SINGLE named
// package internal/bridge over the RED contract authored this cycle in
// completion_cancel_parity_test.go, and asserts the per-test `--- PASS:`
// receipt. The receipt check is load-bearing anti-gaming: `go test -run
// <deleted-test>` matches nothing and exits 0, so an exit-code-only predicate
// would go GREEN if Builder deleted the contract instead of satisfying it
// (cycle-1113 lesson).
//
// Caller proof. Every behavioural test named below drives the REAL production
// entry point — Engine.LaunchArgs → runTmuxREPL → the post-cancel final poll at
// driver_tmux_repl.go:576 — never a detector in isolation. The caller IS the
// fault site here, so a unit-seam test would prove nothing about it. The fakes
// (ctxHonoringTmux, gitEvidenceRunner) reproduce exec.CommandContext's refusal
// to run on a dead ctx, which is what makes these un-passable by a no-op: no
// reordering of detector internals satisfies them, only a usable context does.
//
// Diversity: 001 and 002 each pair a positive (a finished turn completes) with
// an honest negative (an unfinished turn still owes ExitArtifactTimeout, so the
// fix cannot manufacture completion); 003 is a regression axis on the third
// contract; 004 is the verify-and-close of the already-landed sibling task.
package cycle1236

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

// TestC1236_001_StdoutContractGetsBenignTeardownGrace pins AC-1 for
// completion-contract-cancel-parity. Positive: an advisor/router turn that has
// settled on the prompt marker, torn down in the gap before the poll that would
// have completed it, must exit ExitOK — the same grace artifactDetector gets.
// Negative (the anti-no-op axis): a pane still streaming when the teardown lands
// must STILL exit ExitArtifactTimeout, so a fix cannot buy parity by weakening
// the completion condition into "cancelled ⇒ done".
func TestC1236_001_StdoutContractGetsBenignTeardownGrace(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout",
		"TestTmuxREPL_StdoutContract_CancelWhileStreaming_StillTimesOut",
	)
	if !ok {
		report(t, "the stdout completion contract is still starved on the wait loop's final "+
			"post-cancel poll — CapturePane cannot fork tmux on a dead ctx, so a finished "+
			"router/advisor turn is laundered into ExitArtifactTimeout (or the parity fix "+
			"manufactured completion for a turn that never settled)", missing, out)
	}
}

// TestC1236_002_GitEvidenceContractGetsBenignTeardownGrace pins AC-2. Same
// asymmetry, other detector: the phase commits its evidence and the orchestrator
// cancels in the SAME poll gap, so the post-cancel poll is the first look that
// could ever observe the HEAD advance — and `git rev-parse` cannot start on a
// dead ctx. Negative: HEAD that never advanced, carrying a commit with no
// verifying Evolve-Phase trailer, must still time out.
func TestC1236_002_GitEvidenceContractGetsBenignTeardownGrace(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout",
		"TestTmuxREPL_GitContract_CancelWithoutEvidenceCommit_StillTimesOut",
	)
	if !ok {
		report(t, "the git-evidence completion contract is still starved on the final post-cancel "+
			"poll — deps.Runner cannot fork git on a dead ctx, so a verified evidence commit is "+
			"reported as no-completion (or the fix completes a phase that committed nothing)", missing, out)
	}
}

// TestC1236_003_ArtifactContractNotRegressedByTheFix is the regression axis, and
// the reason this cycle is not a one-line change. artifactDetector's
// benign-teardown short-circuit is keyed on ctx.Err() != nil; handing the final
// poll a live context silently disarms it, and the detector then demands a
// stability window it cannot accrue in one call. Finality must be signalled
// explicitly. These two guards — the wait-loop-level completion and the
// detector-level short-circuit with its own paired negative — must both survive
// the re-keying.
func TestC1236_003_ArtifactContractNotRegressedByTheFix(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout",
		"TestTmuxREPL_CancelWithoutDeliverable_StillTimesOut",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
	)
	if !ok {
		report(t, "fixing the parity gap regressed the contract that already worked: the artifact "+
			"benign-teardown short-circuit no longer fires (a live final ctx disarmed the "+
			"ctx.Err() key without an explicit finality signal replacing it)", missing, out)
	}
}

// TestC1236_004_ArtifactCrossPollDebounceStillHolds closes the lane's second
// top_n task, artifact-ready-crosspoll-debounce, which is ALREADY LANDED at HEAD
// (const artifactStableTicks, completion.go:38-43, plus the (size, mtime)
// cross-poll window at :216-232, folded in from cycle-1233 by 841e676f).
// Re-implementing it would stack a second counter on d.stable and double every
// artifact phase's completion latency for no defect closed, so the acceptance
// criterion here is VERIFY, not build: the landed contract — positive settle,
// both negative axes (still-growing file; same-size rewrite that a size-only key
// is blind to), and its reachability from the production wait loop — must still
// hold at the end of this cycle. It is expected GREEN from the first run; its
// job is to fail loudly if this cycle's edits to the shared completion.go
// disturb the sibling contract.
func TestC1236_004_ArtifactCrossPollDebounceStillHolds(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
	)
	if !ok {
		report(t, "the already-landed artifact cross-poll stability window no longer holds — this "+
			"cycle's edits to the shared completion.go disturbed the sibling contract "+
			"(first-sight completion is back, the stability key went size-only, or the window "+
			"is no longer reached from the production wait loop)", missing, out)
	}
}
