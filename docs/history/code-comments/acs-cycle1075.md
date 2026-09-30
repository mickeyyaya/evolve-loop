# Comment history: `acs/cycle1075`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1075/predicates_test.go:3` — above `package cycle1075`

```text
// Package cycle1075 materialises the cycle-1075 acceptance criteria for the two
// fleet-scoped `loop-batch-chaining` tasks pinned to this lane:
//
//   - chain-boundary-loop  → the outer batch-chaining loop + its stop conditions
//   - chain-policy-flag    → `--until-inbox-empty` CLI opt-in + policy `chain` block
//
// Predicate strategy — every predicate EXERCISES the system under test (the
// cycle-85 degenerate-predicate ban forbids source-greps as the load-bearing
// assertion):
//
//   - 001 runs the REAL `evolve loop` binary via `go run` in a throwaway project
//     and reads the emitted --dry-run config JSON: with `--until-inbox-empty` the
//     resolved config must report chain mode ON; WITHOUT it (the negative axis)
//     the same invocation must report it OFF. A no-op that ignores the flag fails.
//   - 002 CALLS the policy loader (`policy.Load` → `Policy.ChainConfig()`) against
//     temp policy.json fixtures: an explicit `chain` block must be honoured, an
//     absent block must fall back to a positive compiled default cap, and a
//     zero/negative `max_batches` (the edge axis) must never yield a 0 cap that
//     would disable chaining outright.
//   - 003 runs the fake-runner chain tests in ./cmd/evolve as a subprocess and
//     requires all four boundary stop conditions (inbox-empty clean exit, quota
//     wall → checkpoint+defer instead of relaunch, max_batches cap, `.evolve/
//     loop-stop` operator brake) to be covered by PASSING named tests.
//   - 004 runs the fleet-width-preserved-across-batches regression test — a
//     naive per-batch re-init silently regresses lane width, so it gets its own
//     predicate per the standing width commitment.
//
// 003/004 shell out to `go test` rather than importing the code because
// go/cmd/evolve is `package main` and cannot be imported. The predicates assert
// on the runner's PASS lines for specifically-named behaviours, so an empty or
// filtered-out run ("no tests to run") is a failure, not a silent pass.
```
