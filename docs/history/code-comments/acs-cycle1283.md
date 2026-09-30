# Comment history: `acs/cycle1283`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1283/predicates_test.go:3` — above `package cycle1283`

```text
// Package cycle1283 materialises the acceptance criteria for the single
// fleet-scoped task pinned to this lane: `retro-fleet-stale-worktree-fallback`
// — the verified-OPEN half of the cycle-1255 CRITICAL that the
// 1255→1268→1270→1272 salvage chain progressively narrowed to the EMPTY-worktree
// shape and then sealed as "verified closed" in the CHANGELOG (68322bdf).
//
// The defect. A torn-down (or never-provisioned) fleet lane leaves
// cs.ActiveWorktree NON-EMPTY but pointing at a directory that no longer exists
// (cyclerun.go:456 is the sole assignment). retroWorktree's original guard fired
// only on the empty string, so the dead path passed through verbatim into the
// BridgeRequest, where the fleet driver's isDir() check refuses the launch
// (driver_tmux_repl.go, ExitBadFlags, stderr only, no error return) — the lane
// loses its retrospective entirely. A failure in the failure-handler.
//
// Predicate strategy — why these drive Phase.Run and not retroWorktree.
// retroWorktree is unexported, and a predicate that called it would prove only
// that a helper behaves; the value the guard actually reads is
// core.BridgeRequest.Worktree, produced by the exported Run method that the
// orchestrator dispatches (cyclerun_dispatch.go builds the PhaseRequest from
// cr.cs.ActiveWorktree and hands it to the registered PhaseRunner). Every
// predicate here therefore constructs the phase through its real constructor,
// calls Run, and asserts on the BridgeRequest a recording bridge captured —
// the production value on the production path. None of them greps source
// (the cycle-85 degenerate-predicate ban); the sole file assertion is 005,
// which checks an operator-mandated DOC deliverable, declared with a waiver.
//
// Discrimination is proven, not assumed: against the cycle base SHA 9b129565
// (the pre-fix retroWorktree) predicates 001 and 003 FAIL, while 002 and 004 —
// the anti-over-widening axes — stay GREEN. See test-report.md's RED Run Output.
```
