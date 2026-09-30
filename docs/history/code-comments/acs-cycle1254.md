# Comment history: `acs/cycle1254`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1254/predicates_test.go:3` — above `package cycle1254`

```text
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
```

### `go/acs/cycle1254/predicates_test.go:83` — above `func runContract(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

```text
// runContract runs `go test -C <worktree>/go -v -run ^(names...)$ ./internal/bridge/`
// and reports whether EVERY named test both ran and passed.
//
// Two failure shapes are distinguished deliberately:
//   - code < 0 is a genuine "could not launch" and is fatal (cycle-574 lesson);
//     a compile failure in the target package is a NON-ZERO EXIT, not a launch
//     failure, and must surface as a normal RED.
//   - a zero exit with a missing `--- PASS: <name>` receipt means the test is
//     GONE, not that it passed. Reported as a miss, never as a pass.
```

### `go/acs/cycle1254/predicates_test.go:130` — above `func TestC1254_001_ArtifactReadyCrossPollDebounceHolds(t *testing.T) {`

```text
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
```
