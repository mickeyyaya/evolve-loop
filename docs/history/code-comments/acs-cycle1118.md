# Comment history: `acs/cycle1118`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1118/predicates_test.go:3` — above `package cycle1118`

```text
// Package cycle1118 materialises the cycle-1118 acceptance criteria for the single
// fleet-scoped task pinned to this lane:
//
//	fatalpane-persistence-gate → the ADR-0044 C2 fatal-pane fast-fail
//	  (go/internal/bridge/fatalpane.go) matches on RAW pane substrings and fires on
//	  ONE observation, while its sibling quota-wall fast-fail
//	  (exhaustion_persistence.go) has been persistence-gated since the
//	  cycle-254/255/314/641 false-FAIL lineage precisely because a WORKING agent can
//	  render fatal-shaped text into the pane it is being judged on. This cycle closes
//	  the asymmetry: a fatal match must persist for `fatalPanePersistObservations`
//	  CONSECUTIVE checkpoints before it can preempt the reviewer or leave C2
//	  evidence, while a genuinely parked pane still fast-fails at the threshold
//	  observation (no regression of the cycle-262 rescue path).
//
// Predicate strategy (the cycle-85 degenerate-predicate ban):
//
//   - 001/002/003 EXERCISE the gate through the package's behavioural unit tests —
//     the transient/reset class fix, the persistent-still-fires bound, and the
//     shadow would/did parity semantics. Each shelled run REQUIRES a real `--- PASS`
//     line, so a renamed, filtered-away, or deleted test is a FAIL, never a silent
//     green.
//   - 004 is the WIRING proof: the checkpoint loop must own exactly ONE gate
//     instance (a per-call gate can never accumulate a streak — it would leave every
//     behavioural test above green while production stays un-gated) and must reach
//     the seam THROUGH it, with no bare fatalPaneVerdict call left behind. This one
//     reads the loop's AST rather than running it: the fatal-pane branch of the
//     stop-review loop is only reachable through a live tmux session whose liveness
//     projection also decides Busy — which independently suppresses preemption — so
//     an end-to-end run cannot isolate the gate's contribution. Structural, but
//     shape-based (call graph inside the owning function), not a string grep for a
//     magic token.
//   - 005 is the regression axis: fatalPaneVerdict's OWN single-observation contract
//     is unchanged, so the pre-existing fatalpane_test.go / fatalpane_durable_test.go
//     cases must still pass UNMODIFIED.
```
