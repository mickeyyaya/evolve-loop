# Comment history: `acs/cycle1236`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1236/predicates_test.go:3` — above `package cycle1236`

```text
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
```

### `go/acs/cycle1236/predicates_test.go:73` — above `func runContract(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

```text
// runContract runs `go test -C <worktree>/go -v -run ^(names...)$ ./internal/bridge/`
// and reports whether EVERY named test both ran and passed.
//
// Two failure shapes are distinguished deliberately:
//   - code < 0 is a genuine "could not launch" and is fatal (cycle-574 lesson);
//     a compile failure in the target package — the expected RED signal before
//     Builder implements — is a NON-ZERO EXIT, not a launch failure.
//   - a zero exit with a missing `--- PASS: <name>` receipt means the test is
//     gone, not that it passed. That is reported as a miss, not a pass.
```

### `go/acs/cycle1236/predicates_test.go:179` — above `func TestC1236_004_ArtifactCrossPollDebounceStillHolds(t *testing.T) {`

```text
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
```
