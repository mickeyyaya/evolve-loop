//go:build acs

// Package cycle1254 materialises the acceptance criteria for this lane's two
// triage-COMMITTED (## top_n) tasks:
//
//	artifact-ready-crosspoll-debounce  — ALREADY LANDED (841e676f, cycle-1198/1249)
//	completion-contract-cancel-parity  — ALREADY LANDED (cycle-1236, HEAD e469bd6b)
//
// READ THIS BEFORE TREATING A GREEN HERE AS A NO-OP. Both tasks were delivered
// in FULL before this cycle opened. These predicates are therefore
// VERIFY-AND-CLOSE, not a RED contract — the same role cycle-1236's own 004
// played for the debounce sibling it inherited already-landed. They exist so the
// two backlog ids can be consumed against EXECUTED evidence rather than against
// a claim, and so the behaviours stay pinned for this cycle's audit.
//
// Why the lane re-opened closed work. cycle-1254's scout-report Key Findings 1
// and 2 describe the PRE-fix state of both items — its prose tracks the "The
// defect" header comment of go/internal/bridge/completion_cancel_parity_test.go
// (the cycle-1236 RED contract, landed and GREEN) rather than the production
// code beside it. Both claimed gaps are contradicted by the source at HEAD:
//
//	Claim 1: "artifactDetector.poll is a single os.Stat+Size()>0 check on ONE
//	poll tick; there is no cross-poll state at all for this detector."
//	Actual:  completion.go:43 artifactStableTicks = 2, and artifactDetector
//	         carries haveLast/lastPath/lastSize/lastModTime/stable across polls
//	         (completion.go:215-270). The window gates RELOCATION too (cycle-1249).
//
//	Claim 2: "the final poll takes the (already-dead) ctx, so git/stdout phases
//	very likely do NOT get the same cancel-after-completion protection."
//	Actual:  driver_tmux_repl.go:585 calls withFinalPoll (completion.go:62),
//	         which hands the poll a context DETACHED from the cancellation
//	         (context.WithoutCancel), bounded by finalPollGrace, carrying an
//	         explicit finality marker. Parity is covered for all three contracts
//	         by four tests in completion_cancel_parity_test.go.
//
// Predicate strategy. artifactDetector, stdoutDetector, gitEvidenceDetector,
// their poll methods and the wait loop are all UNEXPORTED, so these predicates
// cannot import them. Each instead shells `go test -run -v` at the SINGLE named
// package internal/bridge and asserts the per-test `--- PASS:` receipt. The
// receipt check is load-bearing anti-gaming: `go test -run <deleted-test>`
// matches nothing and exits 0, so an exit-code-only predicate would go GREEN if
// the contract tests were DELETED rather than kept passing (cycle-1113 lesson).
// That is the live risk for a verify-and-close cycle specifically: the only way
// to break these tasks now is to remove their proof.
//
// Caller proof. Every behavioural test named in 002 drives the REAL production
// entry point — Engine.LaunchArgs → runTmuxREPL → the post-cancel final poll at
// driver_tmux_repl.go:585 — never a detector in isolation, and 001 includes
// TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop for the same reason. A seam
// whose only caller is a test proves nothing about the loop that starves it.
//
// Diversity: 001 is the debounce behaviour (positive + growing/rewrite
// negatives + the window-size bound that forbids the artifactStableTicks=1
// degenerate fix); 002 is cancel parity across all three contracts through the
// production caller; 003 isolates the anti-no-op axis shared by both tasks —
// every negative that must still REFUSE completion — so a regression that buys
// "parity" by completing unconditionally names itself instead of hiding inside
// an aggregate.
package cycle1254

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// bridgePkg is the ONE named package these predicates run. Never a `./...`
// sweep: a whole-repo run is the regression suite's job and is a false-red
// generator under fleet load (flaky-predicate-shape, Gate D).
const bridgePkg = "./internal/bridge/"

// goDir is the worktree's go module root — predicates read the CYCLE's source,
// not main's (worktree isolation; acsassert.RepoRoot resolves the worktree).
// Passed as `go test -C` so the run never depends on the process cwd, which
// differs between the main tree, this worktree, and each fleet lane.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runContract runs `go test -C <worktree>/go -v -run ^(names...)$ ./internal/bridge/`
// and reports whether EVERY named test both ran and passed.
//
// Two failure shapes are distinguished deliberately:
//   - code < 0 is a genuine "could not launch" and is fatal (cycle-574 lesson);
//     a compile failure in the target package is a NON-ZERO EXIT, not a launch
//     failure, and must surface as a normal RED.
//   - a zero exit with a missing `--- PASS: <name>` receipt means the test is
//     GONE, not that it passed. Reported as a miss, never as a pass.
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

// TestC1254_001_ArtifactReadyCrossPollDebounceHolds closes
// artifact-ready-crosspoll-debounce. The task's acceptance bar is that
// artifactDetector declares an artifact finished only after it has STOPPED
// CHANGING across consecutive poll ticks — which is what makes the doc claim at
// deliverable.go:180 ("mid-write truncation is closed at the SOURCE by the
// bridge artifact-ready cross-poll debounce") true rather than accidental.
//
// The named set covers every axis the task specifies: the multi-tick settle
// (ReadyOnlyAfterCrossPollStability), the growing-file negative
// (NotReadyWhileArtifactStillGrowing), the content-blind edge where size is
// unchanged but mtime moved (NotReadyOnSameSizeRewrite — size alone would
// false-complete an equal-length fix-up Edit), the cycle-1249 requirement that
// the window GATES relocation rather than following it (the three Relocation*
// tests — relocating on first sight copies a partial file into the canonical
// path and then removes the source the agent is still appending to), the
// production caller (ArtifactDebounceWiredIntoWaitLoop), and the degenerate-fix
// bound (StableTicks_IsAMeaningfulWindow: a window of 1 is not a debounce, and
// a window above 3 taxes every phase ~2s per extra tick).
func TestC1254_001_ArtifactReadyCrossPollDebounceHolds(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
		"TestArtifactDetector_RelocationNoteSurvivesUntilStable",
		"TestArtifactDetector_RelocationDeferredWhileFallbackStillGrowing",
		"TestArtifactDetector_RelocationHappensOnceFallbackSettles",
		"TestArtifactDetector_RelocatedCompleteFallbackStillCompletes",
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
		"TestArtifactStableTicks_IsAMeaningfulWindow",
	)
	if !ok {
		report(t, "the artifact-ready cross-poll debounce no longer holds — an artifact is being "+
			"declared finished from a single observation (or the stability window stopped gating "+
			"relocation), which re-opens the mid-write truncated-read the deliverable.go:180 doc "+
			"claims is closed at the source", missing, out)
	}
}

// TestC1254_002_CompletionContractCancelParityHolds closes
// completion-contract-cancel-parity. The wait loop takes ONE final completion
// poll when its context is cancelled, so a session that finished at the buzzer
// is not laundered into ExitArtifactTimeout — and that single call site
// dispatches to all THREE completionDetector strategies. The task's bar is that
// the grace is real for every contract, not just the file-stat one.
//
// The three positives assert the grace (stdout after the REPL settled, git after
// a verifying evidence commit, artifact after the deliverable hit disk); each is
// driven end-to-end through Engine.LaunchArgs, and the stdout/git fakes
// reproduce exec.CommandContext's refusal to run on a dead ctx, so no reordering
// of detector internals can satisfy them — only the detached, finalPollGrace
// -bounded context withFinalPoll supplies. The artifact case is named here as
// the coupling guard: the naive "just pass a live ctx" fix silently disarms
// artifactDetector's short-circuit and regresses the one contract that already
// worked, which is why finality is signalled by an explicit marker instead of
// being inferred from ctx.Err().
func TestC1254_002_CompletionContractCancelParityHolds(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout",
		"TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout",
		"TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout",
	)
	if !ok {
		report(t, "the benign-teardown grace is not contract-agnostic — a DELIVERED phase torn down "+
			"at the finish line is being laundered into ExitArtifactTimeout on at least one "+
			"completion contract (the final post-cancel poll cannot fork tmux/git on a dead ctx, "+
			"or the finality marker artifactDetector keys on was disarmed)", missing, out)
	}
}

// TestC1254_003_CancelGraceAndDebounceRefuseFalseCompletion is the anti-no-op
// axis for BOTH tasks, isolated so a regression names itself.
//
// Every mechanism above trades strictness for grace, and each has a degenerate
// "fix" that greens its positives while destroying its purpose: complete
// whenever the context is cancelled, or complete on first sight. This predicate
// pins the refusals. An unfinished stdout turn (pane still streaming), a git
// phase whose HEAD never advanced and carries no verifying Evolve-Phase trailer,
// and a cancel with no deliverable on disk must ALL still exit
// ExitArtifactTimeout; a growing or same-size-rewritten artifact must still
// report not-ready. A false-complete here is strictly worse than the timeout it
// replaces — it certifies a phase that produced nothing.
func TestC1254_003_CancelGraceAndDebounceRefuseFalseCompletion(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestTmuxREPL_StdoutContract_CancelWhileStreaming_StillTimesOut",
		"TestTmuxREPL_GitContract_CancelWithoutEvidenceCommit_StillTimesOut",
		"TestTmuxREPL_CancelWithoutDeliverable_StillTimesOut",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
	)
	if !ok {
		report(t, "a completion contract now MANUFACTURES completion — an unfinished phase "+
			"(streaming pane / no verifying commit / no deliverable) or an artifact still being "+
			"written is being certified as done, which is a false-PASS generator strictly worse "+
			"than the timeout the grace was added to prevent", missing, out)
	}
}
